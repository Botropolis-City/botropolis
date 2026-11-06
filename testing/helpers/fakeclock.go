package helpers

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"botropolis.city/botropolis/pkg/daemon"
)

// FakeClock is a clock that only moves when a test advances it, firing
// each timer that comes due on the way, so however long a step takes on
// a loaded machine it cannot open a gap the test did not make. It is
// what the daemon's release timer and idle hold read in tests (items
// 205 and 346).
type FakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeTimer
}

type fakeTimer struct {
	at      time.Time
	f       func()
	stopped atomic.Bool
}

func (t *fakeTimer) Stop() bool { return !t.stopped.Swap(true) }

// NewFakeClock is a clock reading now until advanced.
func NewFakeClock(now time.Time) *FakeClock {
	return &FakeClock{now: now}
}

func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *FakeClock) AfterFunc(d time.Duration, f func()) daemon.Stopper {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{at: c.now.Add(d), f: f}
	c.timers = append(c.timers, t)
	return t
}

// Advance moves the clock by by, firing every timer due by then, the
// earliest first.
func (c *FakeClock) Advance(by time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(by)
	var due, kept []*fakeTimer
	for _, t := range c.timers {
		switch {
		case t.stopped.Load():
		case !t.at.After(c.now):
			due = append(due, t)
		default:
			kept = append(kept, t)
		}
	}
	c.timers = kept
	c.mu.Unlock()
	sort.Slice(due, func(i, j int) bool { return due[i].at.Before(due[j].at) })
	for _, t := range due {
		if !t.stopped.Swap(true) {
			t.f()
		}
	}
}
