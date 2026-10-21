package render

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// arrow is a pointer's outline in UI pixels, its tip at the origin.
var arrow = [][2]float32{{0, 0}, {0, 18}, {4.5, 14}, {8, 21.5}, {11, 20}, {7.5, 13}, {13, 13}}

// rippleFor is how long the ring under a scripted click spreads.
const rippleFor = 0.45

// drawPointer draws the scripted pointer into a recording, which has no
// cursor of its own: headless frames are the game's image, and a click
// nobody can see is a cut nobody can follow.
func (g *Game) drawPointer(screen *ebiten.Image) {
	if !g.play.shown {
		return
	}
	scale := float32(g.theme.Scale)
	at := g.play.pointer
	x, y := float32(at.X), float32(at.Y)
	since := float64(g.recorded)/liveTPS - g.clickedAt
	if g.clickedAt > 0 && since >= 0 && since < rippleFor {
		p := since / rippleFor
		alpha := uint8(200 * (1 - p))
		g.strokeCircle(screen, x, y, float32(6+22*p)*scale, 2.5*scale, color.NRGBA{R: 255, G: 255, B: 255, A: alpha})
	}
	pts := g.paint.Points()
	for _, pt := range arrow {
		pts = append(pts, city.Point{X: float64(x + pt[0]*scale), Y: float64(y + pt[1]*scale)})
	}
	g.paint.Fill(screen, pts, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	g.paint.Stroke(screen, pts, math.Max(1.2, 1.4*float64(scale)), true, color.NRGBA{R: 20, G: 22, B: 28, A: 255})
	g.paint.Keep(pts)
}
