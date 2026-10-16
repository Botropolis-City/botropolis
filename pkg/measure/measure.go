// Package measure counts the work the city does on the paths that make a
// task slow: files opened, bytes read, rebuilds, scans. The acceptance
// tests read the counts back to assert how much work a task takes and how
// it grows, which no machine's load can change. Item 91.
//
// The counters are process-wide, so a test that asserts work reads the
// change across its task and does not run in parallel with another that
// does. Nothing in the app shows them.
package measure

import "sync/atomic"

type Counter int

const (
	TranscriptsOpened Counter = iota
	BytesRead
	IndexRebuilds
	SessionScans
	counters
)

var names = [counters]string{"transcripts opened", "bytes read", "index rebuilds", "session scans"}

func (c Counter) String() string { return names[c] }

var counts [counters]atomic.Int64

func Count(c Counter) { counts[c].Add(1) }

func Add(c Counter, n int64) { counts[c].Add(n) }

func Read(c Counter) int64 { return counts[c].Load() }

// Work is the counters' readings, with the ones at zero left out.
type Work map[Counter]int64

// Take reads every counter.
func Take() Work {
	w := Work{}
	for c := Counter(0); c < counters; c++ {
		if n := Read(c); n != 0 {
			w[c] = n
		}
	}
	return w
}

// Since is the work done between an earlier reading and this one.
func (w Work) Since(before Work) Work {
	d := Work{}
	for c, n := range w {
		if n != before[c] {
			d[c] = n - before[c]
		}
	}
	return d
}
