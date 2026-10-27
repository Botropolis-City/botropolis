package render

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// drawSearch shows the filter the finder left under the strip, with how
// much of the map it keeps. While the finder is open the panel shows it
// instead.
func (g *Game) drawSearch(screen *ebiten.Image, top, width float64) {
	if g.query == "" || g.finding() {
		g.mu.Lock()
		g.searchPlate = city.Rect{}
		g.mu.Unlock()
		return
	}
	th := g.theme
	line := fmt.Sprintf("/ %s   %d of %d", g.query, g.scene.Matching(), len(g.scene.City().Buildings()))
	plate := ui.LayoutPlate(th, line, city.Point{X: 2 * th.Grid(), Y: top + th.Grid()}, ui.Body, g.faces.Measure)
	// Kept so a click lands on the plate that was drawn. Bug 43.
	g.mu.Lock()
	g.searchPlate = plate.Rect
	g.mu.Unlock()
	g.roundRect(screen, plate.Rect, th.Radius()/2, th.Palette.Panel)
	g.text(screen, plate.TextAt, line, plate.Size, th.Palette.Text)
}

// dimmed is the tint for a building the search leaves out.
func dimmed() *ebiten.ColorScale { return &dim }

// dim is the one tint every dimmed thing shares, as nothing changes a
// tint it is handed.
var dim = func() ebiten.ColorScale {
	var tint ebiten.ColorScale
	tint.Scale(0.35, 0.35, 0.4, 0.35)
	return tint
}()

// clickSearch opens the finder on the filter by pressing the key that
// opens it. The line already reads "/", so the click presses what the
// reader can see.
func (g *Game) clickSearch(cursor city.Point) bool {
	g.mu.Lock()
	plate := g.searchPlate
	g.mu.Unlock()
	if plate.Area() == 0 || !plate.Contains(cursor) {
		return false
	}
	g.clicked = ebiten.KeySlash
	return true
}
