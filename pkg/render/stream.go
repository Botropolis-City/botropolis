package render

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/ui"
)

// streamView is whether the column is up: shown unless it was hidden,
// and the hide is remembered (item 88); in a window too narrow for it,
// hidden unless its key showed it this run (item 100).
func (g *Game) streamView() (ui.StreamView, bool) {
	v := ui.StreamView{Asked: g.streamAsked}
	if l := g.scene.Layout(); l != nil {
		v.Hidden = l.StreamHidden
	}
	return v, ui.StreamFits(g.theme, g.scene.Size().X)
}

func (g *Game) streamShown() bool {
	v, fits := g.streamView()
	return v.Shown(fits)
}

// toggleStream hides or shows the column and says which.
func (g *Game) toggleStream() {
	l := g.scene.Layout()
	if l == nil {
		return
	}
	v, fits := g.streamView()
	v = v.Toggle(fits)
	l.StreamHidden, g.streamAsked = v.Hidden, v.Asked
	if v.Shown(fits) {
		g.SetStatus("stream shown")
	} else {
		g.SetStatus("stream hidden; " + g.findChords().Stream.String() + " shows it")
	}
}

// drawStream lays the panel in its corner and reserves its room, so the
// map is never under it (item 100). It reports the panel's left edge,
// where the cards must stop.
func (g *Game) drawStream(screen *ebiten.Image, width, height, top, footer float64) float64 {
	if !g.streamShown() {
		g.scene.SetCornerChrome(city.Rect{})
		g.mu.Lock()
		g.streamLaid = ui.Stream{}
		g.mu.Unlock()
		return width
	}
	th := g.theme
	area := ui.StreamArea(th, ui.StreamCorner, width, height, top, footer)
	g.scene.SetCornerChrome(area)
	list := g.scene.Events()
	g.mu.Lock()
	g.streamSkip = ui.FollowStream(g.streamSkip, g.streamSeen, len(list))
	g.streamSeen = len(list)
	s := ui.LayoutStream(th, list, area, g.faces.Measure, g.streamSkip)
	g.streamSkip, g.streamLaid = s.Skip, s
	g.mu.Unlock()
	solid := th.Palette.Panel
	solid.A = 0xff
	fill := func(r city.Rect, c color.NRGBA) {
		vector.FillRect(screen, float32(r.Min.X), float32(r.Min.Y), float32(r.Width()), float32(r.Height()), c, false)
	}
	fill(area, solid)
	fill(city.RectAt(area.Min.X, area.Min.Y, th.Hairline(), area.Height()), th.Palette.Hairline)
	edge := area.Max.Y - th.Hairline()
	if ui.StreamCorner == ui.BottomRight {
		edge = area.Min.Y
	}
	fill(city.RectAt(area.Min.X, edge, area.Width(), th.Hairline()), th.Palette.Hairline)
	g.run(screen, s.Title, th.Palette.Text)
	g.run(screen, s.Empty, th.Palette.Dim)
	g.run(screen, s.Newer, th.Palette.Text)
	for _, r := range s.Rows {
		tone := th.Palette.Dim
		switch r.Event.Kind {
		case events.NeedsYou:
			tone = th.Palette.NeedsYou
		case events.Error:
			tone = th.Palette.Error
		}
		g.run(screen, r.Time, th.Palette.Dim)
		for _, line := range r.Lines {
			g.run(screen, line, tone)
		}
	}
	if s.Thumb.Area() > 0 {
		fill(s.Track, th.Palette.Hairline)
		fill(s.Thumb, th.Palette.Dim)
	}
	return area.Min.X
}

// clickStream takes a press on the panel: on the bar it starts a drag,
// on the newer mark it goes back to the newest, and on an entry it
// flies the camera to the session it is about.
func (g *Game) clickStream(cursor city.Point) bool {
	g.mu.Lock()
	s := g.streamLaid
	g.mu.Unlock()
	if !s.Rect.Contains(cursor) {
		return false
	}
	if s.Thumb.Area() > 0 && s.Track.Inset(-g.theme.Grid()/2).Contains(cursor) {
		g.streamDrag = true
		g.dragStream(cursor)
		return true
	}
	if s.NewerRect.Contains(cursor) {
		g.mu.Lock()
		g.streamSkip = 0
		g.mu.Unlock()
		return true
	}
	if e, ok := s.Hit(cursor); ok && !g.scene.FlyToEvent(e) {
		g.SetStatus(e.Title + " is no longer on the map")
	}
	return true
}

// dragStream scrolls the panel to where the bar is dragged.
func (g *Game) dragStream(cursor city.Point) {
	g.mu.Lock()
	g.streamSkip = g.streamLaid.SkipAt(cursor.Y)
	g.mu.Unlock()
}

// wheelStream scrolls the panel by whole entries while the pointer is
// over it, and reports whether it took the wheel; elsewhere the wheel
// zooms the map (item 100).
func (g *Game) wheelStream(cursor city.Point, notches float64) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.streamLaid.Rect.Contains(cursor) {
		return false
	}
	g.streamWheel += notches
	whole := math.Trunc(g.streamWheel)
	g.streamWheel -= whole
	g.streamSkip = max(g.streamSkip-int(whole), 0)
	return true
}
