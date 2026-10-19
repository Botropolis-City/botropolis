package claude

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
)

func ReadTranscriptWindow(path string, window int64) (Transcript, error) {
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

	tail := make([]byte, window)
	if _, err := f.Seek(info.Size()-window, io.SeekStart); err != nil {
		return Transcript{}, err
	}
	n, err = io.ReadFull(f, tail)
	if err != nil && err != io.ErrUnexpectedEOF {
		return Transcript{}, err
	}
	tail = tail[:n]
	if cut := bytes.IndexByte(tail, '\n'); cut >= 0 {
		tail = tail[cut+1:]
	} else {
		tail = nil
	}

	scan := newTranscriptScan(path, false)
	scan.transcript.Partial = true
	scanLines(scan, bytes.NewReader(head))
	scanLines(scan, bytes.NewReader(tail))
	t := scan.finish()
	t.Usage.Partial = true
	return t, nil
}

func newTranscriptScan(path string, sidechainIsMain bool) *transcriptScan {
	return &transcriptScan{
		transcript:    Transcript{Path: path},
		seenMessages:  map[string]bool{},
		sidechainMain: sidechainIsMain,
	}
}

func scanLines(scan *transcriptScan, r io.Reader) {
	scanner := bufio.NewScanner(countedReader{r})
	scanner.Buffer(nil, maxTranscriptLine)
	for scanner.Scan() {
		scan.line(scanner.Bytes())
	}
}

// line reads one line into the scan.
func (s *transcriptScan) line(raw []byte) {
	rec, present, ok := decodeLine(raw)
	switch {
	case !present:
	case !ok:
		s.transcript.Malformed++
	default:
		s.apply(rec)
	}
}

// decodeLine is the record on a line, whether the line holds anything,
// and whether that parsed. A torn write can leave a run of NUL bytes
// ahead of a record; the record behind them is still good.
func decodeLine(raw []byte) (rec transcriptLineJSON, present, ok bool) {
	line := bytes.TrimLeft(raw, "\x00")
	if len(line) == 0 {
		return rec, false, false
	}
	err := json.Unmarshal(line, &rec)
	return rec, true, err == nil
}

func (s *transcriptScan) finish() Transcript {
	s.transcript.IsBridgeStub = s.records > 0 && s.bridgeRecords == s.records
	s.transcript.Tail = s.tail.finish()
	if s.transcript.Recap.Text != "" {
		s.transcript.Recap.PromptsSince = s.transcript.Tail.Prompts - s.recapPrompts
	}
	return s.transcript
}
