package render

import "fmt"

// clipReport says how many frames a recording wrote and how much of the
// city's clock they span, so a clip's speed can be checked rather than
// judged by eye. Item 113.
func (g *Game) clipReport() string {
	every := max(g.recordEvery, 1)
	written := (g.recorded + every - 1) / every
	var ticks uint64
	if g.recorded > 0 {
		ticks = g.clipTo - g.clipFrom + 1
	}
	return fmt.Sprintf("recorded %d frames: %.3f s of the city's clock", written, float64(ticks)/liveTPS)
}

// startTick begins an update, and reports whether to run it. While
// recording an update waits until the last one has been drawn, because
// the clip counts draws and the city's clock counts updates: on
// software GL Ebitengine ran several updates per draw, and the city ran
// about three times ahead of its clip. Item 113.
func (g *Game) startTick() bool {
	if (g.record != "" || g.share.recording()) && g.ticks != g.painted {
		return false
	}
	g.ticks++
	return true
}

// startPaint begins a draw, and reports whether to draw at all. A draw
// with nothing new is skipped, and while recording that holds too, so a
// display that draws twice an update does not count either toward the
// clip; a screenshot draws regardless, to settle.
func (g *Game) startPaint() bool {
	if !g.fresh() && g.screenshot == "" {
		return false
	}
	g.painted = g.ticks
	return true
}
