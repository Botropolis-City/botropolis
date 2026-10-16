package render

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// The key to the views, drawn over the map.
//
// The nine views were reachable only by keys somebody had to tell you
// about. This says what there is, what each one asks, which one you are
// in, and takes a click — so the map explains its own modes rather than
// relying on the reader having read something else.

func (g *Game) drawViewKey(screen *ebiten.Image, width, height float64) {
	k := ui.LayoutViewKey(g.theme, width, height, g.scene.Lens(), g.faces.Measure)
	// Kept so a click hit-tests the rows that were actually drawn rather
	// than a second layout computed from a second source of the window
	// size. The two agree today — LayoutF feeds both — and that is
	// exactly the kind of agreement that stops being true quietly.
	g.mu.Lock()
	g.viewKeyRows = k
	g.mu.Unlock()
	th := g.theme
	vector.FillRect(screen, 0, 0, float32(width), float32(height), colorScrim, false)
	g.roundPanel(screen, k.Rect)
	g.run(screen, k.Title, th.Palette.Text)
	g.run(screen, k.Subtitle, th.Palette.Dim)
	for _, r := range k.Rows {
		if r.Current {
			// The view you are in is marked rather than merely listed:
			// a menu that cannot say where you are is half a menu.
			g.roundRect(screen, r.Row, th.Radius()/2, th.Palette.Hairline)
		}
		g.roundRect(screen, r.Chip, th.Radius()/2, th.Palette.Hairline)
		g.text(screen, r.KeyAt, r.Key, k.Size, th.Palette.Text)
		name := th.Palette.Text
		if r.Current {
			name = th.Palette.Accent
		}
		g.text(screen, r.NameAt, r.Name, k.Size, name)
		g.text(screen, r.QuestionAt, r.Question, k.Size, th.Palette.Dim)
	}
	g.text(screen, k.Note.At, k.Note.Text, k.Size, th.Palette.Dim)
}

// clickViewKey chooses a view when the key is open and the click lands
// on a row, and swallows every other click while it is up so a menu
// cannot be clicked through onto the map behind it.
func (g *Game) clickViewKey(cursor city.Point) bool {
	if !g.viewKey {
		return false
	}
	g.mu.Lock()
	k := g.viewKeyRows
	g.mu.Unlock()
	if l, ok := k.Hit(cursor); ok {
		g.selectLens(l)
	}
	g.viewKey = false
	return true
}
