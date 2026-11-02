// Package memlog writes the memory a city holds itself, its images and
// its live Go heap, for a check that cannot read the process's: under
// llvmpipe a whole-process reading counts memory the C allocator keeps
// after the driver has freed it (item 318). Only the devtools build
// makes a log, so a release never collects for it.
package memlog

import (
	"fmt"
	"os"
	"runtime"
	"time"
)

// every is how often a log samples.
const every = 5 * time.Second

// Sources are what a log reads of the city: what its images take, and
// the slack its record holds and the half of it, if any, that broke the
// rule it is cut back by.
type Sources struct {
	Images  func() int64
	Records func() (slack int64, broken string)
}

// Log samples a city's memory at most once every 5 s.
type Log struct {
	path    string
	sources Sources
	heap    func() uint64
	now     func() time.Time
	last    time.Time
}

// New is a log written to path, nil when none is asked for or the build
// makes none.
func New(path string, s Sources) *Log {
	if !enabled || path == "" {
		return nil
	}
	return &Log{path: path, sources: s, heap: liveHeap, now: time.Now}
}

// Sample appends a line of the images and the live heap when one is due.
func (l *Log) Sample() {
	if l == nil {
		return
	}
	now := l.now()
	if !l.last.IsZero() && now.Sub(l.last) < every {
		return
	}
	l.last = now
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	line := fmt.Sprintf("images=%d heap=%d", l.sources.Images(), l.heap())
	if l.sources.Records != nil {
		slack, broken := l.sources.Records()
		line += fmt.Sprintf(" slack=%d", slack)
		if broken != "" {
			line += " broken=" + broken
		}
	}
	_, _ = fmt.Fprintln(f, line)
}

// liveHeap is the Go heap's live bytes: what is allocated right after a
// collection.
func liveHeap() uint64 {
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ms.HeapAlloc
}
