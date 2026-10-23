package daemon

import (
	"time"

	"github.com/fsnotify/fsnotify"
)

// SetAddWatch replaces how the daemon adds a watch, so a test can have
// one fail.
func SetAddWatch(d *Daemon, add func(*fsnotify.Watcher, string) error) {
	d.addWatch = add
}

// SetThreadCPU replaces how the daemon reads a rescan's CPU time, so a
// test can say what each costs.
func SetThreadCPU(d *Daemon, cpu func() time.Duration) {
	d.threadCPU = cpu
}

// ReleaseClock is a clock for the release timer that a test moves.
type ReleaseClock = releaseClock

// Stopper is a timer that can be stopped.
type Stopper = stopper

// SetReleaseClock replaces the clock the release timer reads and arms,
// dropping what the old one had armed.
func SetReleaseClock(d *Daemon, clock ReleaseClock) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.settle != nil {
		d.settle.Stop()
	}
	d.releaseClock, d.settle, d.unreleased = clock, nil, time.Time{}
}

// PaceAfter is how long the watcher waits after a rescan that cost cost.
func PaceAfter(cost time.Duration) time.Duration {
	return paceAfter(cost)
}

// ThreadCPU is the CPU time the calling thread has used.
func ThreadCPU() time.Duration {
	return threadCPU()
}
