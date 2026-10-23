package state

import "time"

// SetStatsClock sets the clock the stats pass reads to decide when to
// ingest everything again.
func SetStatsClock(l *Loader, clock func() time.Time) {
	l.stored.clock = clock
}

// StatsPassRunning is whether a stats pass is running.
func StatsPassRunning(l *Loader) bool {
	l.stored.mu.Lock()
	defer l.stored.mu.Unlock()
	return l.stored.running
}
