package render

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"botropolis.city/botropolis/pkg/city"
)

// Everything drawn in front of a lit window cuts it out of the night's
// lights, and not only a sprite (item 195): a district's sign, a
// building's own signage over its own windows, its beacon and flags, a
// scaffold, a monument, are painted as meshes, stamps and words, and the
// windows behind them showed through. The painter tells the lights of
// each shape it draws, and each painted word is told here, in depth
// order, at its own alpha, so a half-transparent shape dims the light
// behind it rather than putting it out.

// cutKind is what a light op cuts with, when it cuts.
type cutKind uint8

const (
	cutNone cutKind = iota
	// cutMesh is painted meshes, cutVertices and cutIndices[from:to].
	cutMesh
	// cutStamp is a disc's stamp, src drawn with geo at alpha.
	cutStamp
	// cutRing is a ring the painter filled by its fallback,
	// cutPoints[from:to] at alpha.
	cutRing
	// cutWords is painted words, words in face drawn with geo at alpha.
	cutWords
)

// lightCutter is the painter's Cutter while the night's lights are
// being noted.
type lightCutter struct{ g *Game }

func (c lightCutter) CutMesh(vertices []ebiten.Vertex, indices []uint32) {
	c.g.cutMesh(vertices, indices)
}

func (c lightCutter) CutImage(img *ebiten.Image, op *ebiten.DrawImageOptions) {
	g := c.g
	if !g.cutting() {
		return
	}
	b := img.Bounds()
	box := transformedBox(op.GeoM, float64(b.Dx()), float64(b.Dy()), 0, 0)
	if !g.coversLight(box) {
		return
	}
	g.lightOps = append(g.lightOps, lightOp{cut: cutStamp, src: img, geo: op.GeoM, alpha: op.ColorScale.A(), box: box})
	g.cuts++
}

func (c lightCutter) CutRing(pts []city.Point, alpha float32) {
	g := c.g
	if !g.cutting() || len(pts) < 3 {
		return
	}
	box := image.Rectangle{}
	for i, p := range pts {
		r := pixelAround(p.X, p.Y)
		if i == 0 {
			box = r
		} else {
			box = box.Union(r)
		}
	}
	if !g.coversLight(box) {
		return
	}
	from := len(g.cutPoints)
	g.cutPoints = append(g.cutPoints, pts...)
	g.lightOps = append(g.lightOps, lightOp{cut: cutRing, from: from, to: len(g.cutPoints), alpha: alpha, box: box})
	g.cuts++
}

// cutting is whether what is drawn now cuts the lights: while they are
// being noted, once any window is lit, and unless painting is uncut.
func (g *Game) cutting() bool {
	return g.lightsOn && !g.paintUncut && len(g.litRects) > 0
}

// coversLight is whether a box on screen covers any window lit so far.
func (g *Game) coversLight(box image.Rectangle) bool {
	if !box.Overlaps(g.litBounds) {
		return false
	}
	for _, r := range g.litRects {
		if box.Overlaps(r) {
			return true
		}
	}
	return false
}

// cutMesh notes painted meshes as a cut, joined to every mesh cut noted
// since the last window was, so the shapes between two windows are one
// draw: the cut buffers only grow with mesh cuts, so theirs is one range.
func (g *Game) cutMesh(vertices []ebiten.Vertex, indices []uint32) {
	if !g.cutting() || len(indices) == 0 {
		return
	}
	box := image.Rectangle{}
	for i, v := range vertices {
		r := pixelAround(float64(v.DstX), float64(v.DstY))
		if i == 0 {
			box = r
		} else {
			box = box.Union(r)
		}
	}
	if !g.coversLight(box) {
		return
	}
	base := uint32(len(g.cutVertices))
	g.cutVertices = append(g.cutVertices, vertices...)
	from := len(g.cutIndices)
	for _, i := range indices {
		g.cutIndices = append(g.cutIndices, i+base)
	}
	g.cuts++
	if g.meshCut >= 0 {
		g.lightOps[g.meshCut].to = len(g.cutIndices)
		g.lightOps[g.meshCut].box = g.lightOps[g.meshCut].box.Union(box)
		return
	}
	g.meshCut = len(g.lightOps)
	g.lightOps = append(g.lightOps, lightOp{cut: cutMesh, from: from, to: len(g.cutIndices), box: box})
}

// cutWords notes painted words as a cut.
func (g *Game) cutWords(words string, face text.Face, geo ebiten.GeoM, alpha float32) {
	if !g.cutting() || words == "" {
		return
	}
	w, h := text.Measure(words, face, 0)
	box := transformedBox(geo, w, h, 0, 0)
	if !g.coversLight(box) {
		return
	}
	g.lightOps = append(g.lightOps, lightOp{cut: cutWords, words: words, face: face, geo: geo, alpha: alpha, box: box})
	g.cuts++
}

// layCut cuts one op's sprite or shapes out of the lights.
func (g *Game) layCut(dst *ebiten.Image, op lightOp) {
	if op.punch {
		dst.DrawImage(op.src, &ebiten.DrawImageOptions{GeoM: op.geo, Blend: ebiten.BlendDestinationOut, Filter: ebiten.FilterLinear})
		return
	}
	switch op.cut {
	case cutMesh:
		g.paint.CutTriangles(dst, g.cutVertices, g.cutIndices[op.from:op.to])
	case cutStamp:
		o := &ebiten.DrawImageOptions{GeoM: op.geo, Blend: ebiten.BlendDestinationOut, Filter: ebiten.FilterLinear, DisableMipmaps: true}
		o.ColorScale.ScaleAlpha(op.alpha)
		dst.DrawImage(op.src, o)
	case cutRing:
		g.paint.CutPath(dst, g.cutPoints[op.from:op.to], op.alpha)
	case cutWords:
		o := &text.DrawOptions{}
		o.GeoM = op.geo
		o.Blend = ebiten.BlendDestinationOut
		o.Filter = ebiten.FilterLinear
		o.ColorScale.ScaleAlpha(op.alpha)
		text.Draw(dst, op.words, op.face, o)
	}
}

// transformedBox is the screen box around a w×h rectangle at (x, y)
// moved by geo.
func transformedBox(geo ebiten.GeoM, w, h, x, y float64) image.Rectangle {
	box := image.Rectangle{}
	for i, c := range [][2]float64{{x, y}, {x + w, y}, {x, y + h}, {x + w, y + h}} {
		px, py := geo.Apply(c[0], c[1])
		r := pixelAround(px, py)
		if i == 0 {
			box = r
		} else {
			box = box.Union(r)
		}
	}
	return box
}

// pixelAround is the pixels a point touches, and one more each way.
func pixelAround(x, y float64) image.Rectangle {
	return image.Rect(int(math.Floor(x)), int(math.Floor(y)), int(math.Ceil(x))+1, int(math.Ceil(y))+1)
}
