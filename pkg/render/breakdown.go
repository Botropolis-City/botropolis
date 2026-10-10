package render

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// drawBreakdown is the plant's panel: the city's spend in a window by
// model, project and session, with the week's series above it.
func (g *Game) drawBreakdown(screen *ebiten.Image, width, height float64) {
	th := g.theme
	c := g.scene.City()
	vector.FillRect(screen, 0, 0, float32(width), float32(height), colorScrim, false)
	titles := map[string]string{}
	for _, b := range c.Buildings() {
		titles[b.Session.ID] = b.Card(c.Time).Title
	}
	p := ui.LayoutBreakdown(th, width, height, c.Breakdown(g.window), c.Series(g.window), titles, g.faces.Measure)
	g.roundPanel(screen, p.Rect)
	g.run(screen, p.Title, th.Palette.Text)
	for i, b := range p.Windows {
		colour := th.Palette.Dim
		if city.Window(i) == p.Current {
			colour = th.Palette.Accent
		}
		g.roundRect(screen, b.Rect, th.Radius()/2, th.Palette.Hairline)
		g.run(screen, b.Label, colour)
	}
	g.sparkline(screen, p.Spark)
	for _, col := range p.Columns {
		g.run(screen, col.Head, th.Palette.Dim)
		for _, row := range col.Rows {
			g.run(screen, row.Label, th.Palette.Text)
			g.run(screen, row.Value, th.Palette.Dim)
		}
	}
	g.mu.Lock()
	g.breakdownLayout = p
	g.mu.Unlock()
}

// clickBreakdown answers a click while the panel is open: a window
// button switches the window, anything else closes the panel.
func (g *Game) clickBreakdown(at city.Point) bool {
	if !g.breakdown {
		return false
	}
	g.mu.Lock()
	p := g.breakdownLayout
	g.mu.Unlock()
	if w, ok := p.HitWindow(at); ok {
		g.window = w
		return true
	}
	if !p.Rect.Contains(at) {
		g.breakdown = false
	}
	return true
}

// breakdownKeys answers the keyboard while the panel is open; it
// reports whether the panel stays open.
func (g *Game) breakdownKeys() bool {
	just := g.just
	switch {
	case just(ebiten.KeyEscape), just(ebiten.KeyX), just(ebiten.KeyQ):
		return false
	case just(ebiten.KeyDigit1):
		g.window = city.LastHour
	case just(ebiten.KeyDigit2):
		g.window = city.LastDay
	case just(ebiten.KeyDigit3):
		g.window = city.LastWeek
	case just(ebiten.KeyArrowRight):
		g.window = city.Window((int(g.window) + 1) % 3)
	case just(ebiten.KeyArrowLeft):
		g.window = city.Window((int(g.window) + 2) % 3)
	case just(ebiten.KeyP):
		g.snap = screenshotPath(timeNow())
	}
	return true
}
