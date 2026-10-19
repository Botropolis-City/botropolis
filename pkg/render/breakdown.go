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
	if g.rangeOn && g.rangeReadable() {
		g.drawRanges(screen, width, height)
		return
	}
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
	g.run(screen, p.Subtitle, th.Palette.Dim)
	row := ui.LayoutWindowRow(th, p.Windows[0].Rect.Min, p.Rect.Max.X-3*th.Grid()-p.Windows[0].Rect.Min.X, g.rowState(), g.faces.Measure)
	g.drawWindowRow(screen, row)
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
	g.rowLayout = row
	g.mu.Unlock()
}

// clickBreakdown answers a click while the panel is open: a window
// button switches the window, anything else closes the panel.
func (g *Game) clickBreakdown(at city.Point) bool {
	if !g.breakdown {
		return false
	}
	if g.rangeOn && g.rangeReadable() {
		return g.clickRanges(at)
	}
	g.mu.Lock()
	p, row := g.breakdownLayout, g.rowLayout
	g.mu.Unlock()
	if c, ok := row.Hit(at); ok {
		g.choose(c)
		return true
	}
	if w, ok := p.HitWindow(at); ok {
		g.choose(ui.WindowChoice{Window: w})
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
	if g.rangeOn && g.rangeReadable() {
		return g.rangeKeys()
	}
	just := g.just
	switch {
	case just(ebiten.KeyEscape), just(ebiten.KeyX), just(ebiten.KeyQ):
		return false
	case just(ebiten.KeyDigit1):
		g.choose(ui.WindowChoice{Window: city.LastHour})
	case just(ebiten.KeyDigit2):
		g.choose(ui.WindowChoice{Window: city.LastDay})
	case just(ebiten.KeyDigit3):
		g.choose(ui.WindowChoice{Window: city.LastWeek})
	case just(ebiten.KeyArrowRight):
		g.choose(ui.WindowChoice{Window: g.window}.Next(g.rangeReadable()))
	case just(ebiten.KeyArrowLeft):
		g.choose(ui.WindowChoice{Window: g.window}.Prev(g.rangeReadable()))
	case just(ebiten.KeyP):
		g.snap = screenshotPath(timeNow())
	}
	return true
}
