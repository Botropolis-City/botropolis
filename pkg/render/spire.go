package render

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

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

// drawSpire draws a building's spire, if its session has one.
func (g *Game) drawSpire(screen *ebiten.Image, cam *city.Camera, b *city.Building) {
	if b.Session.Spire <= 0 || b.Packed {
		return
	}
	foot := cam.WorldToScreen(b.Rect.Center())
	base := city.Point{X: foot.X, Y: foot.Y - g.kitTop(cam, buildingPiece(b))}
	s := spireOf(base, storeyHeight(cam.Zoom), b.Session.Spire)
	for _, seg := range s.segments {
		vector.StrokeLine(screen, float32(seg.a.X), float32(seg.a.Y), float32(seg.b.X), float32(seg.b.Y), float32(seg.width+1.5), colorSpireEdge, true)
		vector.StrokeLine(screen, float32(seg.a.X), float32(seg.a.Y), float32(seg.b.X), float32(seg.b.Y), float32(seg.width), colorSpire, true)
		vector.FillCircle(screen, float32(seg.b.X), float32(seg.b.Y), float32(seg.width*0.8), colorSpireEdge, true)
	}
	if s.beacon {
		r := float32(max(2, storeyHeight(cam.Zoom)*0.12))
		glow := colorBeacon
		glow.A = 0x60
		vector.FillCircle(screen, float32(s.tip.X), float32(s.tip.Y), r*2.2, glow, true)
		vector.FillCircle(screen, float32(s.tip.X), float32(s.tip.Y), r, colorBeacon, true)
	}
}
