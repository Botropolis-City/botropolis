package claude

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"time"

	"botropolis.city/botropolis/pkg/measure"
)

const sinceChunk = 64 << 10

// ReadTranscriptSince reads a transcript's head window, for its title,
// and everything from the start of since's hour to its end, so its
// hourly usage from that hour on is what a whole read gives. Item 94.
//
// The walk back from the end goes past that hour while it meets lines
// of a message already read, because a message is bucketed at its first
// line, and until it has met a cost-state record, because the cost is
// the last one's. A walk that reaches the head reads the file whole.
func ReadTranscriptSince(path string, window int64, since time.Time) (Transcript, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Transcript{}, err
	}
	if info.Size() <= 2*window {
		return readTranscriptFile(path, false)
	}
	f, err := openTranscript(path)
	if err != nil {
		return Transcript{}, err
	}
	defer func() { _ = f.Close() }()

	head := make([]byte, window)
	n, err := io.ReadFull(f, head)
	if err != nil && err != io.ErrUnexpectedEOF {
		return Transcript{}, err
	}
	head = head[:n]
	if cut := bytes.LastIndexByte(head, '\n'); cut >= 0 {
		head = head[:cut+1]
	} else {
		head = nil
	}

	hourStart := since.Unix() / 3600 * 3600
	scan := newTranscriptScan(path, false)
	if !beganBefore(head, hourStart) {
		return scanWhole(scan, f)
	}
	start, err := walkBack(f, int64(len(head)), info.Size(), hourStart)
	if err != nil {
		return Transcript{}, err
	}
	if start <= int64(len(head)) {
		return scanWhole(scan, f)
	}
	scan.transcript.Partial = true
	scanLines(scan, bytes.NewReader(head))
	scanLines(scan, io.NewSectionReader(f, start, info.Size()-start))
	t := scan.finish()
	t.Usage.Partial = true
	return t, nil
}

func scanWhole(scan *transcriptScan, f *os.File) (Transcript, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return Transcript{}, err
	}
	scanLines(scan, f)
	return scan.finish(), nil
}

type sinceLineJSON struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Message   struct {
		ID string `json:"id"`
	} `json:"message"`
}

// beganBefore is whether the head's first timestamp is before the
// hour, or it has none; a transcript that began inside the hour is
// read whole without a walk.
func beganBefore(head []byte, hourStart int64) bool {
	for line := range bytes.SplitSeq(head, []byte{'\n'}) {
		var rec sinceLineJSON
		if json.Unmarshal(bytes.TrimLeft(line, "\x00"), &rec) != nil {
			continue
		}
		if ts, ok := parseTimestamp(rec.Timestamp); ok {
			return ts.Unix() < hourStart
		}
	}
	return true
}

// walkBack is the offset the read from the end starts at, or floor or
// less when it reaches the head. A line longer than a chunk is kept in
// pieces until its start is found, so it is copied once.
func walkBack(f *os.File, floor, size, hourStart int64) (int64, error) {
	walk := backWalk{ids: map[string]bool{}, hourStart: hourStart}
	var pieces [][]byte
	for pos := size; pos > floor; {
		from := max(floor, pos-sinceChunk)
		chunk := make([]byte, pos-from)
		if _, err := f.ReadAt(chunk, from); err != nil && err != io.EOF {
			return 0, err
		}
		measure.Add(measure.BytesRead, int64(len(chunk)))
		end := len(chunk)
		for {
			i := bytes.LastIndexByte(chunk[:end], '\n')
			if i < 0 {
				break
			}
			line := chunk[i+1 : end]
			if pieces != nil {
				line = bytes.Join(append([][]byte{line}, pieces...), nil)
				pieces = nil
			}
			if walk.stopsAt(line) {
				return from + int64(i) + 1 + int64(len(line)) + 1, nil
			}
			end = i
		}
		pieces = append([][]byte{chunk[:end]}, pieces...)
		pos = from
	}
	if line := bytes.Join(pieces, nil); walk.stopsAt(line) {
		return floor + int64(len(line)) + 1, nil
	}
	return floor, nil
}

type backWalk struct {
	ids       map[string]bool
	costSeen  bool
	hourStart int64
	// searching is a walk past the hour for a cost-state record. The
	// hours it passes are before the plant's, so their lines are only
	// searched, not decoded, and the walk ends at the record.
	searching bool
	done      bool
}

var costStateType = []byte(`"cost-state"`)

// stopsAt is whether the walk ends at this line, leaving it out.
func (w *backWalk) stopsAt(line []byte) bool {
	if w.done {
		return true
	}
	line = bytes.TrimLeft(line, "\x00")
	if len(line) == 0 || (w.searching && !bytes.Contains(line, costStateType)) {
		return false
	}
	var rec sinceLineJSON
	if json.Unmarshal(line, &rec) != nil {
		return false
	}
	if rec.Type == "cost-state" {
		w.costSeen, w.done = true, w.searching
		return false
	}
	if ts, ok := parseTimestamp(rec.Timestamp); ok && ts.Unix() < w.hourStart {
		sameMessage := rec.Type == "assistant" && rec.Message.ID != "" && w.ids[rec.Message.ID]
		if !sameMessage {
			if w.costSeen {
				return true
			}
			w.searching = true
		}
	}
	if rec.Type == "assistant" && rec.Message.ID != "" {
		w.ids[rec.Message.ID] = true
	}
	return false
}
