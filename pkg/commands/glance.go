package commands

import (
	"sync"
	"time"
)

// Glance holds a feed off the daemon while the city cannot be seen
// (item 197). Decoding the daemon's whole snapshot each second is most
// of what a hidden city spends, so a hidden feed lets its subscription
// go and attaches only every Every, for one snapshot and the events
// since the last it saw, which keeps the window's title, and whether
// someone needs you, at most that far behind. Shown again, it attaches
// at once.
type Glance struct {
	Every time.Duration

	mu      sync.Mutex
	hidden  bool
	changed chan struct{}
}

// NewGlance is a glance every so often while hidden; the city starts
// shown.
func NewGlance(every time.Duration) *Glance {
	return &Glance{Every: every, changed: make(chan struct{})}
}

// Hide says whether the city can be seen.
func (g *Glance) Hide(hidden bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if hidden == g.hidden {
		return
	}
	g.hidden = hidden
	close(g.changed)
	g.changed = make(chan struct{})
}

// state is whether the city is hidden, and a channel closed when that
// changes. A nil glance is a city always shown.
func (g *Glance) state() (bool, <-chan struct{}) {
	if g == nil {
		return false, nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.hidden, g.changed
}
