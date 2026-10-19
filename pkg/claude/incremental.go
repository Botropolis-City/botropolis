package claude

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"maps"
	"os"
	"slices"

	"botropolis.city/botropolis/pkg/measure"
)

// headCheckBytes is how much of a transcript's start is compared with
// what was read before, to tell a file rewritten in place from one that
// only grew.
const headCheckBytes = 4096

// TranscriptReader reads one transcript as it grows: it keeps what it
// derived and where it stopped, and on each Read reads only what was
// appended since. It reads the file whole again when the file is not
// the one it read, shrank, or changed before where it stopped, so it
// derives exactly what ReadTranscript derives. Item 141.
type TranscriptReader struct {
	path            string
	sidechainIsMain bool
	scan            *transcriptScan
	info            os.FileInfo
	// offset is the end of the last line applied.
	offset int64
	head   []byte
	// open is a last line applied without its newline, which the next
	// byte written must end.
	open bool
	// halted is a scan stopped at a line longer than the cap, as a whole
	// read stops, so nothing after it is read.
	halted bool
}

func NewTranscriptReader(path string) *TranscriptReader {
	return &TranscriptReader{path: path}
}

func (r *TranscriptReader) Read() (Transcript, error) {
	f, err := openTranscript(r.path)
	if err != nil {
		return Transcript{}, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return Transcript{}, err
	}
	if r.scan == nil || !r.continues(f, info) {
		r.reset()
	}
	r.info = info
	torn := false
	if !r.halted {
		if torn, err = r.readOn(f); err != nil {
			r.scan = nil
			return Transcript{}, err
		}
	}
	t := r.scan.finish().clone()
	if torn {
		t.Malformed++
	}
	return t, nil
}

func (r *TranscriptReader) reset() {
	r.scan = newTranscriptScan(r.path, r.sidechainIsMain)
	r.offset, r.head, r.open, r.halted = 0, nil, false, false
}

// continues reports whether the file is the one read before, only
// grown since: the same file, no shorter than it was, with the same
// head, and a line's end where the last read stopped.
func (r *TranscriptReader) continues(f *os.File, info os.FileInfo) bool {
	if r.info == nil || !os.SameFile(r.info, info) || info.Size() < r.info.Size() {
		return false
	}
	n := min(int64(len(r.head)), r.offset)
	head := make([]byte, n)
	if !readAt(f, head, 0) || !bytes.Equal(head, r.head[:n]) {
		return false
	}
	if r.halted || r.offset == 0 {
		return true
	}
	at := r.offset - 1
	if r.open {
		if info.Size() == r.offset {
			return true
		}
		at = r.offset
	}
	b := make([]byte, 1)
	return readAt(f, b, at) && b[0] == '\n'
}

func readAt(f *os.File, p []byte, off int64) bool {
	n, err := f.ReadAt(p, off)
	measure.Add(measure.BytesRead, int64(n))
	return err == nil || (errors.Is(err, io.EOF) && n == len(p))
}

// readOn scans from the offset to the end of the file. A last line
// without its newline is applied when it parses, as a whole read would;
// when it does not, it is a write still under way, so it is left to be
// read again and reported torn.
func (r *TranscriptReader) readOn(f *os.File) (torn bool, err error) {
	if _, err := f.Seek(r.offset, io.SeekStart); err != nil {
		return false, err
	}
	var advance int
	var ended bool
	scanner := bufio.NewScanner(countedReader{f})
	scanner.Buffer(nil, maxTranscriptLine)
	scanner.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		a, token, err := bufio.ScanLines(data, atEOF)
		if a > 0 {
			advance, ended = a, data[a-1] == '\n'
		}
		return a, token, err
	})
	pos := r.offset
	for scanner.Scan() {
		pos += int64(advance)
		if ended {
			r.scan.line(scanner.Bytes())
			r.offset, r.open = pos, false
			continue
		}
		rec, present, ok := decodeLine(scanner.Bytes())
		switch {
		case !present:
		case !ok:
			torn = true
		default:
			r.scan.apply(rec)
			r.offset, r.open = pos, true
		}
	}
	if err := scanner.Err(); errors.Is(err, bufio.ErrTooLong) {
		r.halted = true
	} else if err != nil {
		return false, err
	}
	if want := min(headCheckBytes, r.offset); int64(len(r.head)) < want {
		head := make([]byte, want)
		if !readAt(f, head, 0) {
			return false, io.ErrUnexpectedEOF
		}
		r.head = head
	}
	return torn, nil
}

// clone is a transcript sharing nothing a scan goes on to change.
func (t Transcript) clone() Transcript {
	t.Hourly = maps.Clone(t.Hourly)
	t.Messages = maps.Clone(t.Messages)
	t.Touches = maps.Clone(t.Touches)
	t.MCPCalls = maps.Clone(t.MCPCalls)
	t.SkillCalls = maps.Clone(t.SkillCalls)
	t.Background = maps.Clone(t.Background)
	t.Cost.Models = maps.Clone(t.Cost.Models)
	t.PRs = slices.Clone(t.PRs)
	t.PeerMessages = slices.Clone(t.PeerMessages)
	t.OrderLines = slices.Clone(t.OrderLines)
	return t
}
