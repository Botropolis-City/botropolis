package daemon

import (
	"botropolis.city/botropolis/pkg/measure"
	"time"

	"github.com/fsnotify/fsnotify"
)

// SetAddWatch replaces how the daemon adds a watch, so a test can have
// one fail.
func SetAddWatch(d *Daemon, add func(*fsnotify.Watcher, string) error) {
	d.addWatch = add
}

// SetRescanMeter replaces how the daemon reads what a rescan cost, so a
// test can say what each costs.
func SetRescanMeter(d *Daemon, meter func(func()) measure.Cost) {
	d.meter = meter
}

// WatchClock is the time the watcher reads, and how it waits.
type WatchClock = watchClock

// SetWatchClock replaces the clock the watcher reads and waits on.
func SetWatchClock(d *Daemon, clock WatchClock) {
	d.watchClock = clock
}

// PaceAfter is how long the watcher waits after a rescan that cost cost.
func PaceAfter(cost time.Duration) time.Duration {
	return paceAfter(cost)
}
