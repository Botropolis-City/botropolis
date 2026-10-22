package render

import (
	"fmt"
	"os"
	"time"
)

// openCeiling is the longest a recording waits for its history to be
// read before it opens the city on what it holds. A staged demo home is
// read in well under a second; the ceiling is there so a history that
// never reads cannot hold a film or a frame test for ever (item 202).
const openCeiling = 30 * time.Second

// holdUnopened takes the feed's newest snapshot for a recording whose
// city has not opened, without ticking, and reports whether it is still
// to wait. A recording's clock waits for each tick to be painted, and
// an unopened city paints nothing of the city, so a wait inside the
// tick would never end.
func (g *Game) holdUnopened(now time.Time) bool {
	if g.scene == nil || !g.capturing() || g.ticks == 0 || !g.scene.WaitingForRead() {
		return false
	}
	g.mu.Lock()
	pending := g.pending
	g.pending = nil
	g.mu.Unlock()
	g.takeSnapshot(pending)
	g.openAtCeiling(now)
	return !g.scene.Opened()
}

// openAtCeiling opens a recording's city on the snapshot it holds once
// it has waited openCeiling for a read history, says so on stderr, and
// starts the shot.
func (g *Game) openAtCeiling(now time.Time) {
	if g.scene == nil || !g.capturing() || g.scene.Opened() || now.Sub(g.started) < openCeiling {
		return
	}
	g.scene.OpenAnyway()
	if !g.scene.Opened() {
		return
	}
	g.openedEarly = true
	fmt.Fprintf(os.Stderr, "botropolis: the history was not read within %s, so the recording opens on what was read\n", openCeiling)
	if g.shotFrames == 0 {
		g.shotFrames = 1
	}
}
