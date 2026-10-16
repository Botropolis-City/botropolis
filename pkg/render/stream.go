package render

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/ui"
)

// streamShown is whether the stream's column is up: shown unless it was
// hidden, and the hide is remembered (item 88).
func (g *Game) streamShown() bool {
	l := g.scene.Layout()
	return l == nil || !l.StreamHidden
}

// toggleStream hides or shows the column and says which.
func (g *Game) toggleStream() {
	l := g.scene.Layout()
	if l == nil {
		return
	}
	l.StreamHidden = !l.StreamHidden
	if l.StreamHidden {
		g.SetStatus("stream hidden; " + g.findChords().Stream.String() + " shows it")
	} else {
		g.SetStatus("stream shown")
	}
}

// drawStream lays the column down the right and reserves its room, so
// the map sits beside it and never under it. It reports the column's
// left edge, where the cards must stop.
func (g *Game) drawStream(screen *ebiten.Image, width, height, top, footer float64) float64 {
	if !g.streamShown() {
		g.scene.SetRightChrome(0)
		g.mu.Lock()
		g.streamLaid = ui.Stream{}
		g.mu.Unlock()
		return width
	}
	th := g.theme
	area := ui.StreamArea(th, width, height, top, footer)
	g.scene.SetRightChrome(area.Width())
	s := ui.LayoutStream(th, g.scene.Events(), area, g.faces.Measure)
	g.mu.Lock()
	g.streamLaid = s
	g.mu.Unlock()
	solid := th.Palette.Panel
	solid.A = 0xff
	vector.FillRect(screen, float32(area.Min.X), float32(area.Min.Y), float32(area.Width()), float32(area.Height()), solid, false)
	vector.FillRect(screen, float32(area.Min.X), float32(area.Min.Y), float32(th.Hairline()), float32(area.Height()), th.Palette.Hairline, false)
	g.run(screen, s.Title, th.Palette.Text)
	g.run(screen, s.Empty, th.Palette.Dim)
	for _, r := range s.Rows {
		tone := th.Palette.Dim
		switch r.Event.Kind {
		case events.NeedsYou:
			tone = th.Palette.NeedsYou
		case events.Error:
			tone = th.Palette.Error
		}
		g.run(screen, r.Time, th.Palette.Dim)
		g.run(screen, r.Text, tone)
	}
	return area.Min.X
}

// clickStream flies the camera to the session an entry is about.
func (g *Game) clickStream(cursor city.Point) bool {
	g.mu.Lock()
	s := g.streamLaid
	g.mu.Unlock()
	if !s.Rect.Contains(cursor) {
		return false
	}
	if e, ok := s.Hit(cursor); ok && !g.scene.FlyToEvent(e) {
		g.SetStatus(e.Title + " is no longer on the map")
	}
	return true
}
