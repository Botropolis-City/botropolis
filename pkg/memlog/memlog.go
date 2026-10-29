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

// Log samples a city's memory at most once every 5 s.
type Log struct {
	path   string
	images func() int64
	heap   func() uint64
	now    func() time.Time
	last   time.Time
}

// New is a log written to path, nil when none is asked for or the build
// makes none; images reads what the city's images take.
func New(path string, images func() int64) *Log {
	if !enabled || path == "" {
		return nil
	}
	return &Log{path: path, images: images, heap: liveHeap, now: time.Now}
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
	_, _ = fmt.Fprintf(f, "images=%d heap=%d\n", l.images(), l.heap())
}

// liveHeap is the Go heap's live bytes: what is allocated right after a
// collection.
func liveHeap() uint64 {
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ms.HeapAlloc
}
