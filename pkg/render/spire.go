package render

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// A session heavier than the top line of the user's scale raises a thin
// spire from its roof: a segment per doubling, up to six, with a beacon
// at the tip. Item 109.

var (
	colorSpire     = color.NRGBA{0x8a, 0x92, 0x9c, 0xff}
	colorSpireEdge = color.NRGBA{0x3a, 0x3f, 0x46, 0xff}
	colorBeacon    = color.NRGBA{0xff, 0x5a, 0x3c, 0xff}
)

// kindStorey is a storey of the city's own buildings in kit units, as
// tools/render-sprites/buildings.py cuts them.
const kindStorey = 0.36

// storeyHeight is how tall a storey stands on screen at a zoom: a kit
// unit is the tile's diagonal across, and a vertical unit is
// foreshortened by the camera's thirty degrees.
func storeyHeight(zoom float64) float64 {
	return kindStorey * city.IsoTileWidth / math.Sqrt2 * zoom * math.Cos(math.Pi/6)
}

// spireSegment is how tall one segment of a spire is: half a storey, so
// six stay under three storeys.
func spireSegment(storey float64) float64 { return storey * 0.5 }

type spire struct {
	segments []segment
	tip      city.Point
	beacon   bool
}

// spireOf is the spire of n segments rising from base, each a little
// thinner than the one below.
func spireOf(base city.Point, storey float64, n int) spire {
	s := spire{tip: base}
	if n <= 0 {
		return s
	}
	h := spireSegment(storey)
	for i := range n {
		from := city.Point{X: base.X, Y: base.Y - float64(i)*h}
		to := city.Point{X: base.X, Y: base.Y - float64(i+1)*h}
		width := max(1, storey*0.14*(1-float64(i)/float64(n+2)))
		s.segments = append(s.segments, segment{a: from, b: to, width: width})
		s.tip = to
	}
	s.beacon = true
	return s
}

// spireBase is where a spire stands: on the roof, at the centre of the
// building's own footprint, at the height the pipeline found the roof
// there. A piece without one stands it on its sprite's top.
func (g *Game) spireBase(cam *city.Camera, b *city.Building) city.Point {
	foot := g.footOf(cam, b)
	if g.kits != nil {
		if atlas := g.kits.pick(cam.Zoom); atlas != nil {
			if sprite, ok := atlas.Sprite(buildingPiece(b), cam.Heading); ok && sprite.Body.Roof > 0 {
				c := b.Rect.Center()
				c.X += sprite.Body.X * city.CellSize
				at := cam.WorldToScreen(c)
				return city.Point{X: at.X, Y: foot.Y - (at.Y - cam.WorldToScreen(b.Rect.Center()).Y) - sprite.Body.Roof*storeyHeight(cam.Zoom)/kindStorey}
			}
		}
	}
	return city.Point{X: foot.X, Y: foot.Y - g.kitTop(cam, buildingPiece(b))}
}

// spireSegments is how many segments a building's spire has: none for a
// crate or for ruins.
func spireSegments(b *city.Building) int {
	if b.Packed || b.Session.ConversationGone {
		return 0
	}
	return max(0, b.Session.Spire)
}

// drawSpire draws a building's spire, if its session has one.
func (g *Game) drawSpire(screen *ebiten.Image, cam *city.Camera, b *city.Building) {
	n := spireSegments(b)
	if n == 0 {
		return
	}
	s := spireOf(g.spireBase(cam, b), storeyHeight(cam.Zoom), n)
	for _, seg := range s.segments {
		g.strokeLine(screen, float32(seg.a.X), float32(seg.a.Y), float32(seg.b.X), float32(seg.b.Y), float32(seg.width+1.5), colorSpireEdge)
		g.strokeLine(screen, float32(seg.a.X), float32(seg.a.Y), float32(seg.b.X), float32(seg.b.Y), float32(seg.width), colorSpire)
		g.fillCircle(screen, float32(seg.b.X), float32(seg.b.Y), float32(seg.width*0.8), colorSpireEdge)
	}
	if s.beacon {
		r := float32(max(2, storeyHeight(cam.Zoom)*0.12))
		glow := colorBeacon
		glow.A = 0x60
		g.fillCircle(screen, float32(s.tip.X), float32(s.tip.Y), r*2.2, glow)
		g.fillCircle(screen, float32(s.tip.X), float32(s.tip.Y), r, colorBeacon)
	}
}
