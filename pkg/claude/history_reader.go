package claude

import (
	"bytes"
	"encoding/json"
	"errors"
	"hash/crc32"
	"io"
	"maps"
	"os"
	"slices"
	"time"
)

var castagnoli = crc32.MakeTable(crc32.Castagnoli)

// HistoryReader reads the prompt history again and again, parsing only
// the lines appended since it last read (item 158): Claude Code appends
// a line on every prompt in every session. The bytes already parsed are
// checked against a checksum each time, so a history replaced, cut short
// or rewritten in place is read again whole. Everything it keeps is in
// memory; nothing it reads is written anywhere.
type HistoryReader struct {
	// parsed is how many bytes of the file were complete lines, sum
	// their checksum, and history the prompts in them.
	parsed  int64
	sum     uint32
	history map[string][]Prompt
}

func NewHistoryReader() *HistoryReader {
	return &HistoryReader{}
}

// Read is every prompt in the history by session, in the order typed. A
// missing file is an empty history. What it returns is never changed by
// a later Read.
func (r *HistoryReader) Read(path string) (map[string][]Prompt, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		r.parsed, r.sum, r.history = 0, 0, nil
		return map[string][]Prompt{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	if !r.prefixHolds(f) {
		r.parsed, r.sum, r.history = 0, 0, map[string][]Prompt{}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
	}
	rest, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	complete := bytes.LastIndexByte(rest, '\n') + 1
	if complete > 0 {
		r.history = withLines(r.history, rest[:complete])
		r.sum = crc32.Update(r.sum, castagnoli, rest[:complete])
		r.parsed += int64(complete)
	}
	if complete == len(rest) {
		return r.history, nil
	}
	return withLines(r.history, rest[complete:]), nil
}

// prefixHolds is whether the file still begins with the bytes already
// parsed, leaving it positioned after them.
func (r *HistoryReader) prefixHolds(f *os.File) bool {
	if r.history == nil {
		return false
	}
	h := crc32.New(castagnoli)
	n, err := io.CopyN(h, f, r.parsed)
	return err == nil && n == r.parsed && h.Sum32() == r.sum
}

// withLines is history with the prompts in lines added, copying each
// session it adds to so that history itself is left as it was.
func withLines(history map[string][]Prompt, lines []byte) map[string][]Prompt {
	out := maps.Clone(history)
	if out == nil {
		out = map[string][]Prompt{}
	}
	copied := map[string]bool{}
	for len(lines) > 0 {
		line := lines
		if i := bytes.IndexByte(lines, '\n'); i >= 0 {
			line, lines = lines[:i], lines[i+1:]
		} else {
			lines = nil
		}
		id, p, ok := parseHistoryLine(line)
		if !ok {
			continue
		}
		if !copied[id] {
			out[id] = slices.Clip(out[id])
			copied[id] = true
		}
		out[id] = append(out[id], p)
	}
	return out
}

func parseHistoryLine(line []byte) (string, Prompt, bool) {
	line = bytes.TrimLeft(line, "\x00")
	var rec historyLineJSON
	if json.Unmarshal(line, &rec) != nil || rec.SessionID == "" {
		return "", Prompt{}, false
	}
	return rec.SessionID, Prompt{
		Text:   rec.Display,
		Pasted: pastedTexts(rec),
		At:     time.UnixMilli(rec.Timestamp),
		Dir:    rec.Project,
	}, true
}
