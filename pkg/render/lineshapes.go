package render

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// lineShapes draws each group's line in one of the forms item 348 tries:
// its links and outlines in the line's ink over its casing, over the
// city as the lines are, faded but for the focused group, and no
// stations.
func (g *Game) lineShapes(screen *ebiten.Image, cam *city.Camera) {
	width := math.Max(2, 3*cam.Zoom)
	focus := g.scene.FocusedGroup()
	for _, s := range g.scene.Shapes() {
		ink, casing := colorLineInk, colorLineCasing
		if focus != "" && s.Group != focus {
			ink.A, casing.A = lineFaded, lineFaded
		}
		var segments [][2]city.Point
		segments = append(segments, s.Links...)
		for _, o := range s.Outlines {
			segments = append(segments, o...)
		}
		pts := make([][2]city.Point, len(segments))
		for i, seg := range segments {
			pts[i] = [2]city.Point{cam.WorldToScreen(seg[0]), cam.WorldToScreen(seg[1])}
		}
		g.keepOverlay(kindLines, valueHash(struct {
			pts         [][2]city.Point
			ink, casing color.NRGBA
			width       float64
		}{pts, ink, casing, width}), func() {
			for _, p := range pts {
				g.strokeLine(screen, float32(p[0].X), float32(p[0].Y), float32(p[1].X), float32(p[1].Y), float32(width+2), casing)
			}
			for _, p := range pts {
				g.strokeLine(screen, float32(p[0].X), float32(p[0].Y), float32(p[1].X), float32(p[1].Y), float32(width), ink)
			}
		})
	}
}
