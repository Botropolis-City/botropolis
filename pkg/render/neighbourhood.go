package render

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// kerbRise is how tall a neighbourhood's kerb stands, in screen pixels at
// zoom 1: a low wall, so it reads past the buildings beside it (Aria,
// 2026-11-06, item 348).
const kerbRise = 3.0

// kerbWall is the face of the kerb along one edge of a neighbourhood's
// outline, on screen: its foot from one end to the other, then its top
// back.
func kerbWall(cam *city.Camera, edge [2]city.Point) [4]city.Point {
	a, b := cam.WorldToScreen(edge[0]), cam.WorldToScreen(edge[1])
	rise := kerbRise * cam.Zoom
	return [4]city.Point{a, b, {X: b.X, Y: b.Y - rise}, {X: a.X, Y: a.Y - rise}}
}

// neighbourhoods paints every district's neighbourhood kerbs into the
// ground layer: a low wall in the lines' light ink, cased in their dark,
// along each block's outline. No new colour.
func (g *Game) neighbourhoods(screen *ebiten.Image, c *city.City, cam *city.Camera) {
	if g.scene.Kerbs() == city.KerbsOff {
		return
	}
	var walls [][4]city.Point
	for _, d := range c.Districts {
		for _, n := range c.Neighbourhoods(d) {
			for _, e := range n.Edges() {
				walls = append(walls, kerbWall(cam, e))
			}
		}
	}
	if len(walls) == 0 {
		return
	}
	casing := float32(math.Max(1, cam.Zoom))
	for _, w := range walls {
		g.paint.Fill(screen, w[:], colorLineInk)
		g.strokeLine(screen, float32(w[2].X), float32(w[2].Y), float32(w[3].X), float32(w[3].Y), casing, colorLineCasing)
		g.strokeLine(screen, float32(w[0].X), float32(w[0].Y), float32(w[1].X), float32(w[1].Y), casing, colorLineCasing)
	}
}
