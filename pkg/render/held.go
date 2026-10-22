package render

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/state"
)

// What a snapshot costs a city nobody can see (item 197).
//
// Laying the city out is most of what a city that draws nothing spends,
// and the daemon sends a snapshot a second while anyone works. So a city
// that cannot be seen, unmapped or minimised, holds the newest snapshot
// and lays it out once it can be seen again; only its title, which a
// taskbar shows, follows each one. An unfocused city that can be seen is
// a dashboard on another monitor (Aria, 2026-10-21): it lays out every
// snapshot and paints one frame for it, and nothing moves in between.

// takeSnapshot lays out the newest snapshot from the daemon, or holds it
// while the city cannot be seen; a nil pending lays out any snapshot
// still held once it can be.
func (g *Game) takeSnapshot(pending *state.Snapshot) {
	seen := g.capturing() || g.driven || shown()
	if g.glance != nil && (!g.told || g.toldHidden == seen) {
		g.told, g.toldHidden = true, !seen
		g.glance(!seen)
	}
	if pending != nil {
		g.held = pending
	}
	if g.held == nil {
		return
	}
	if !seen {
		if pending != nil {
			g.setTitle(g.scene.HeadlineOf(*pending))
		}
		return
	}
	snapshot := *g.held
	g.held = nil
	at := time.Now()
	g.scene.SetSnapshot(snapshot)
	g.shareSnapshot()
	g.maybeJournal()
	g.followUsage()
	g.setTitle(g.scene.Summary().Headline())
	frames.laidOut(time.Since(at))
	if g.capturing() && g.shotFrames == 0 && g.scene.Opened() {
		g.shotFrames = 1
	}
	g.dashboard = true
}

// setTitle puts the headline in the window's title, which a taskbar shows.
func (g *Game) setTitle(headline string) {
	if title := windowTitle + " — " + headline; title != g.shownTitle {
		g.shownTitle = title
		ebiten.SetWindowTitle(title)
	}
}

// dashboardFrame reports whether a city nobody is watching paints this
// frame: one for each snapshot it laid out, while it can be seen.
func (g *Game) dashboardFrame() bool {
	if !g.dashboard || !shown() {
		return false
	}
	g.dashboard = false
	return true
}
