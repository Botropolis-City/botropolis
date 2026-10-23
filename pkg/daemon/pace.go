package daemon

import (
	"runtime"
	"time"
)

const (
	// rescanShare is the share of a core the watcher's rescans may take
	// on any home: after each, the next waits until it has cost no more
	// than this (the roadmap manager, item 205).
	rescanShare = 0.01
	// maxPace is the longest it waits, so token counts and a session
	// seen only in its transcript are never more than this late; a home
	// whose rescans need longer to hold the share goes over it instead.
	maxPace = 5 * time.Second
)

// paceAfter is how long the watcher waits after a rescan that cost cost
// before the next: long enough for the rescans to hold to their share,
// and no less than a second, as item 196 set, nor more than maxPace.
func paceAfter(cost time.Duration) time.Duration {
	return min(max(pace, time.Duration(float64(cost)/rescanShare)), maxPace)
}

// timedRescan rescans on this goroutine's own thread, and reports what
// that cost the thread, so other work on the machine and the daemon's
// other goroutines do not count against it.
func (d *Daemon) timedRescan() (time.Duration, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	before := d.threadCPU()
	err := d.Rescan()
	return d.threadCPU() - before, err
}
