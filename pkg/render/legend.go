package render

import (
	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// legend draws the row that says what the view's colours are worth.
//
// It is drawn for as long as the view is up, not for a few seconds: a
// colour standing for a number is unreadable without the number, so a
// legend that fades takes the view with it.
func (g *Game) legend(screen *ebiten.Image, l ui.Legend) {
	if l.Rect.Area() == 0 {
		return
	}
	th := g.theme
	g.bar(screen, l.Rect, l.Rect.Min.Y)
	g.run(screen, l.Title, th.Palette.Dim)
	for _, e := range l.Entries {
		g.roundRect(screen, e.Box, th.Radius()/3, e.Swatch)
		g.text(screen, e.LabelAt, e.Label, l.Size, th.Palette.Text)
	}
	// Low carries the ramp's near end, or — when the view has nothing to
	// scale — the sentence that stands in for the whole bar.
	if l.Low.Text != "" {
		g.run(screen, l.Low, th.Palette.Dim)
	}
	if l.Bar.Area() > 0 {
		g.rampBar(screen, l.Bar)
		g.run(screen, l.High, th.Palette.Text)
	}
	if l.Aside.Text != "" {
		g.run(screen, l.Aside, th.Palette.Dim)
	}
}

// rampBar paints the sequential ramp across a rect, a column at a time,
// so the legend shows the same gradient the map does rather than a
// handful of its stops.
func (g *Game) rampBar(screen *ebiten.Image, r city.Rect) {
	w := r.Width()
	if w <= 0 {
		return
	}
	for x := 0.0; x < w; x++ {
		g.paint.Box(screen, float32(r.Min.X+x), float32(r.Min.Y), 1, float32(r.Height()), ui.Sequential.At(x/(w-1)))
	}
}
