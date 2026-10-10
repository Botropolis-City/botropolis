package render

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// searchKeys takes typed characters into the query while the search box
// is open; Escape clears it, Enter keeps it and closes the box. It
// reports whether the box stays open.
func (g *Game) searchKeys() bool {
	just := g.just
	switch {
	case just(ebiten.KeyEscape):
		g.query = ""
		g.scene.SetFilter("")
		return false
	case just(ebiten.KeyEnter):
		return false
	case just(ebiten.KeyBackspace):
		if n := len([]rune(g.query)); n > 0 {
			g.query = string([]rune(g.query)[:n-1])
		}
	}
	for _, r := range ebiten.AppendInputChars(nil) {
		if r >= ' ' {
			g.query += string(r)
		}
	}
	if g.scriptedRune != 0 {
		g.query += string(g.scriptedRune)
	}
	g.scene.SetFilter(g.query)
	return true
}

// drawSearch shows the query under the strip with how much of the map
// it keeps; while the box is open a caret follows the text.
func (g *Game) drawSearch(screen *ebiten.Image, top, width float64) {
	if g.query == "" && !g.searching {
		return
	}
	th := g.theme
	text := g.query
	if g.searching {
		text += "_"
	}
	all := len(g.scene.City().Buildings())
	line := fmt.Sprintf("/ %s   %d of %d", text, g.scene.Matching(), all)
	if text == "" || text == "_" {
		line = "/ " + text + "   type to filter by title, project, branch, state or model"
	}
	plate := ui.LayoutPlate(th, line, city.Point{X: 2 * th.Grid(), Y: top + th.Grid()}, ui.Body, g.faces.Measure)
	// Kept so a click lands on the plate that was drawn. Bug 43.
	g.mu.Lock()
	g.searchPlate = plate.Rect
	g.mu.Unlock()
	g.roundRect(screen, plate.Rect, th.Radius()/2, th.Palette.Panel)
	g.text(screen, plate.TextAt, line, plate.Size, th.Palette.Text)
}

// dimmed is the tint for a building the search leaves out.
func dimmed() *ebiten.ColorScale {
	tint := &ebiten.ColorScale{}
	tint.Scale(0.35, 0.35, 0.4, 0.35)
	return tint
}

// clickSearch puts the caret back in the filter by pressing the key
// that opens it. The line already reads "/", so the click presses what
// the reader can see.
func (g *Game) clickSearch(cursor city.Point) bool {
	if g.searching {
		return false
	}
	g.mu.Lock()
	plate := g.searchPlate
	g.mu.Unlock()
	if plate.Area() == 0 || !plate.Contains(cursor) {
		return false
	}
	g.clicked = ebiten.KeySlash
	return true
}
