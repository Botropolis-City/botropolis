package render

import (
	"runtime/debug"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// Whether anyone can see the window, and what to do when nobody can.
//
// Focus alone is not the question, and gating on it alone made things
// worse rather than better: i3 holds a window in its scratchpad
// focused while it is unmapped, so a gate on focus never closes — and
// an unmapped window is exactly where the cost runs away, because the
// buffer swap stops blocking on the compositor and the loop free-runs
// as fast as it can. Measured on a desktop, hidden cost 50.3% of a core
// against 35.6% for a window in plain sight.
//
// So the loop is throttled here rather than left to the display: when
// nobody is watching, Update sleeps and Draw paints nothing. That holds
// whatever the window manager thinks focus means and whether or not
// there is a vertical blank to wait for.

const (
	// liveTPS is the tick while someone is watching.
	liveTPS = 30
	// stillTPS is the tick while someone is watching a city in which
	// nothing moves. Ten a second still answers a key or a drag without
	// anyone noticing the difference, and there is no motion left to
	// make choppy — city.Scene.Animating says when that holds.
	stillTPS = 10
	// idleTPS is the tick while nobody is. Clamping it is not enough on
	// its own: the tick governs Update, and it is the frame that runs
	// away, because Draw is called once per display refresh whatever the
	// tick is — measured at sixty a second while Update ran at thirty.
	// So idleSleep is spent in Draw, where it holds the frame rate down
	// whether or not there is a vertical blank left to wait for.
	idleTPS   = 2
	idleSleep = 250 * time.Millisecond
)

// seen is what the window manager says about the window. It is a
// variable so a test can answer for it.
var seen = func() (focused, visible, minimised bool) {
	return ebiten.IsFocused(), ebiten.IsWindowVisible(), ebiten.IsWindowMinimized()
}

// watching reports whether the frame is worth drawing: someone can see
// the window, or a script is capturing it. A scripted frame runs on a
// virtual display with no window manager to focus or map it, and would
// otherwise never be taken.
func watching(scripted bool) bool {
	if scripted {
		return true
	}
	focused, visible, minimised := seen()
	if focused && visible && !minimised {
		return true
	}
	frames.quiet(focused, visible, minimised)
	return false
}

// capturing reports whether this run is writing frames to disk.
func (g *Game) capturing() bool { return g.screenshot != "" || g.record != "" }

// fresh reports whether anything has happened since the last frame was
// painted. Ebitengine calls Draw once per display refresh and Update at
// the tick, so on a sixty-hertz screen half the frames repaint exactly
// what is already on the glass. Leaving those alone is item 2's insight
// one level up: the cheapest drawing is the drawing not done.
func (g *Game) fresh() bool { return g.ticks != g.painted }

// idle throttles the loop when nobody is watching and returns whether
// this frame should be drawn at all.
func (g *Game) idle() bool {
	live := watching(g.capturing() || g.driven)
	tps := idleTPS
	switch {
	case g.record != "":
		// One update per draw; startTick holds any Ebitengine forces
		// on top, as it does on a resize. Item 113.
		tps = ebiten.SyncWithFPS
	case g.capturing() || g.driven:
		tps = liveTPS
	case live && g.scene.Animating():
		tps = liveTPS
	case live:
		tps = stillTPS
	}
	if live != g.live || tps != g.tps {
		wasLive := g.live
		g.live, g.tps = live, tps
		ebiten.SetTPS(tps)
		if wasLive && !live {
			// Going quiet is the moment to hand memory back. An idle
			// app allocates too little to make the collector run, so
			// whatever the last busy stretch peaked at is what a hidden
			// window would otherwise sit on.
			debug.FreeOSMemory()
		}
	}
	return live
}
