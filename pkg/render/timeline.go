package render

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// drawTimeline lists what the map has seen happen (or, after the window
// comes back, what needed you while it was away), with a cursor to jump
// from.
func (g *Game) drawTimeline(screen *ebiten.Image, width, height float64) {
	th := g.theme
	vector.FillRect(screen, 0, 0, float32(width), float32(height), colorScrim, false)
	title, list := "Timeline", g.scene.Events()
	if g.away != nil {
		title, list = "While you were away", g.away
	}
	if g.cursor >= len(list) {
		g.cursor = max(0, len(list)-1)
	}
	tl := ui.LayoutTimeline(th, width, height, title, list, g.cursor, g.faces.Measure)
	g.roundPanel(screen, tl.Rect)
	g.run(screen, tl.Title, th.Palette.Text)
	g.run(screen, tl.Subtitle, th.Palette.Dim)
	for _, row := range tl.Rows {
		if row.Selected {
			g.roundRect(screen, row.Rect.Inset(th.Grid()/2), th.Radius()/2, th.Palette.Hairline)
		}
		if row.Time.Text != "" {
			g.run(screen, row.Time, th.Palette.Dim)
			centre := row.Dot.Center()
			vector.FillCircle(screen, float32(centre.X), float32(centre.Y), float32(row.Dot.Width()/2), th.Color(row.Tone), true)
		}
		g.run(screen, row.Title, th.Palette.Text)
		g.run(screen, row.Detail, th.Palette.Dim)
	}
	g.mu.Lock()
	g.timelineLayout = tl
	g.mu.Unlock()
}

// timelineKeys answers the keyboard while the timeline is open; it
// reports whether it stays open.
func (g *Game) timelineKeys() bool {
	just := g.just
	switch {
	case just(ebiten.KeyEscape), just(ebiten.KeyT), just(ebiten.KeyQ):
		g.away = nil
		return false
	case just(ebiten.KeyArrowDown), just(ebiten.KeyJ):
		g.cursor++
	case just(ebiten.KeyArrowUp), just(ebiten.KeyK):
		g.cursor = max(0, g.cursor-1)
	case just(ebiten.KeyEnter):
		g.jumpToEvent()
		g.away = nil
		return false
	case just(ebiten.KeyP):
		g.snap = screenshotPath(timeNow())
	}
	return true
}

// jumpToEvent selects and centres the session under the cursor.
func (g *Game) jumpToEvent() {
	list := g.scene.Events()
	if g.away != nil {
		list = g.away
	}
	if g.cursor < len(list) && list[g.cursor].SessionID != "" {
		if !g.scene.Select(list[g.cursor].SessionID) {
			g.SetStatus("that session is no longer on the map")
		}
	}
}

// clickTimeline answers a click while the timeline is open: a row jumps
// to its session, anywhere else closes it.
func (g *Game) clickTimeline(at city.Point) bool {
	if !g.timeline {
		return false
	}
	g.mu.Lock()
	tl := g.timelineLayout
	g.mu.Unlock()
	if row, ok := tl.Hit(at); ok && row.SessionID != "" {
		if !g.scene.Select(row.SessionID) {
			g.SetStatus("that session is no longer on the map")
		}
	}
	g.timeline, g.away = false, nil
	return true
}

// watchFocus notices the window losing and regaining focus; on the way
// back it opens the away list when something needed you meanwhile.
func (g *Game) watchFocus() {
	focused := ebiten.IsFocused()
	now := timeNow()
	switch {
	case g.focused && !focused:
		g.blurredAt = now
		g.scene.NoteSeen(now)
		if g.saveState != nil && g.record == "" {
			g.saveState(g.scene.Layout())
		}
	case !g.focused && focused && !g.blurredAt.IsZero():
		g.awaySince = g.blurredAt
	}
	g.focused = focused
	if focused && g.record == "" {
		g.scene.NoteSeen(now)
	}
	// The away list waits for the log to arrive, so a window that was
	// closed shows what happened meanwhile, not an empty panel.
	if focused && g.eventsArrived && !g.awaySince.IsZero() {
		if away := g.scene.Away(g.awaySince); len(away) > 0 && !g.timeline {
			g.away, g.timeline, g.cursor = away, true, 0
		}
		g.awaySince = time.Time{}
	}
}
