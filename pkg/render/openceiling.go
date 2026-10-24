package render

import (
	"fmt"
	"time"
)

// openCeiling is the longest a capture waits for its history to be read.
// A capture reads at full speed (item 240); the ceiling is there so a
// history that never reads cannot hold a film or a frame test for ever
// (item 202).
const openCeiling = 30 * time.Second

// holdUnopened takes the feed's newest snapshot for a capture whose city
// has not opened, without ticking, and reports whether it is still to
// wait. A recording's clock waits for each tick to be painted, and an
// unopened city paints nothing of the city, so a wait inside the tick
// would never end. At the ceiling it fails the capture instead.
func (g *Game) holdUnopened(now time.Time) (bool, error) {
	if g.scene == nil || !g.capturing() || g.ticks == 0 || !g.scene.WaitingForRead() {
		return false, nil
	}
	g.mu.Lock()
	pending := g.pending
	g.pending = nil
	g.mu.Unlock()
	g.takeSnapshot(pending)
	if err := g.unreadAtCeiling(now); err != nil {
		return false, err
	}
	return !g.scene.Opened(), nil
}

// unreadAtCeiling is the error that ends a capture whose city has waited
// openCeiling on a history still unread: a frame taken from a partial
// read would be wrong without saying so (item 240).
func (g *Game) unreadAtCeiling(now time.Time) error {
	if g.scene == nil || !g.capturing() || !g.scene.WaitingForRead() || now.Sub(g.started) < openCeiling {
		return nil
	}
	return fmt.Errorf("the history was not read within %s, so no frame is taken from a partial read", openCeiling)
}
