package state

import "sync"

// loadGate is held for the length of every load, and the statistics
// backfill yields at it, so a rescan never shares the CPU with the
// backfill and a session's state is never late behind it (item 138).
type loadGate struct {
	mu sync.RWMutex
}

func (g *loadGate) hold() func() {
	g.mu.Lock()
	return g.mu.Unlock
}

func (g *loadGate) yield() {
	g.mu.RLock()
	defer g.mu.RUnlock()
}
