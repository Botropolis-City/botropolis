package render

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// These take the arguments of the vector package's anti-aliased calls
// they replace, and draw through the painter: a line or a rectangle as a
// mesh with a one-pixel fringe, a disc from a stamp, with no stencil
// passes (item 191).

func (g *Game) strokeLine(dst *ebiten.Image, x0, y0, x1, y1, width float32, c color.NRGBA) {
	g.paint.Line(dst, float64(x0), float64(y0), float64(x1), float64(y1), float64(width), c)
}

func (g *Game) fillCircle(dst *ebiten.Image, cx, cy, r float32, c color.NRGBA) {
	g.paint.Disc(dst, city.Point{X: float64(cx), Y: float64(cy)}, float64(r), c)
}

func (g *Game) strokeCircle(dst *ebiten.Image, cx, cy, r, width float32, c color.NRGBA) {
	g.paint.Circle(dst, city.Point{X: float64(cx), Y: float64(cy)}, float64(r), float64(width), c)
}

func (g *Game) fillRect(dst *ebiten.Image, x, y, w, h float32, c color.NRGBA) {
	pts := g.rectPoints(x, y, w, h)
	g.paint.Fill(dst, pts, c)
	g.paint.Keep(pts)
}

func (g *Game) strokeRect(dst *ebiten.Image, x, y, w, h, width float32, c color.NRGBA) {
	pts := g.rectPoints(x, y, w, h)
	g.paint.Stroke(dst, pts, float64(width), true, c)
	g.paint.Keep(pts)
}

func (g *Game) rectPoints(x, y, w, h float32) []city.Point {
	x0, y0, x1, y1 := float64(x), float64(y), float64(x+w), float64(y+h)
	return append(g.paint.Points(), city.Point{X: x0, Y: y0}, city.Point{X: x1, Y: y0}, city.Point{X: x1, Y: y1}, city.Point{X: x0, Y: y1})
}

// fillPoints fills a closed ring of screen points.
func (g *Game) fillPoints(dst *ebiten.Image, pts []city.Point, c color.NRGBA) {
	g.paint.Fill(dst, pts, c)
}

// strokePoints strokes an open polyline of screen points.
func (g *Game) strokePoints(dst *ebiten.Image, pts []city.Point, width float32, c color.NRGBA) {
	g.paint.Stroke(dst, pts, float64(width), false, c)
}
