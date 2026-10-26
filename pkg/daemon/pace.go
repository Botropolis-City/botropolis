package daemon

import "time"

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

// meteredRescan rescans on this goroutine's own thread, and reports what
// that cost a core as the cost gates count one, so other work on the
// machine and the daemon's other goroutines do not count against it. A
// rescan's thread CPU ran a third or more under its counted cost on a
// quiet machine, so rescans paced by it took more than their share
// (item 213).
func (d *Daemon) meteredRescan() (time.Duration, error) {
	var err error
	cost := d.meter(func() { err = d.Rescan() })
	return cost.Core(), err
}
