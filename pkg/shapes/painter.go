package shapes

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
)

// Painter draws shapes onto images. Its zero value is ready to use: the
// white pixel and the disc stamps are made the first time they are
// needed, and its buffers are kept between shapes, so drawing allocates
// nothing once they have grown.
type Painter struct {
	mesh  Mesh
	white *ebiten.Image
	discs []*ebiten.Image
	path  vector.Path
	line  [2]city.Point
	pts   []city.Point
	tri   ebiten.DrawTrianglesOptions
	img   ebiten.DrawImageOptions
}

// Points is a buffer of points the painter keeps, emptied, for a caller
// to build a curve in without allocating.
func (p *Painter) Points() []city.Point { return p.pts[:0] }

// Keep hands a grown points buffer back, so the next Points reuses it.
func (p *Painter) Keep(pts []city.Point) { p.pts = pts[:0] }

// Fill fills a closed ring of points.
func (p *Painter) Fill(dst *ebiten.Image, pts []city.Point, c color.NRGBA) {
	p.FillBlend(dst, pts, c, ebiten.BlendSourceOver)
}

// FillBlend fills a closed ring of points with a blend.
func (p *Painter) FillBlend(dst *ebiten.Image, pts []city.Point, c color.NRGBA, blend ebiten.Blend) {
	if !p.mesh.Fill(pts, c) {
		p.fallback(dst, pts, c, blend)
	}
	p.flush(dst, blend)
}

// FillRing fills a ring whose triangulation r remembers between frames.
func (p *Painter) FillRing(dst *ebiten.Image, pts []city.Point, c color.NRGBA, r *Ring) {
	if !p.mesh.FillRing(pts, c, r) {
		p.fallback(dst, pts, c, ebiten.BlendSourceOver)
	}
	p.flush(dst, ebiten.BlendSourceOver)
}

// Stroke strokes a polyline of points width wide; closed joins its last
// point to its first.
func (p *Painter) Stroke(dst *ebiten.Image, pts []city.Point, width float64, closed bool, c color.NRGBA) {
	p.mesh.Stroke(pts, width, closed, c)
	p.flush(dst, ebiten.BlendSourceOver)
}

// AddStroke adds a stroke to what the next Flush draws, so many strokes
// go down in one draw.
func (p *Painter) AddStroke(pts []city.Point, width float64, closed bool, c color.NRGBA) {
	p.mesh.Stroke(pts, width, closed, c)
}

// AddLine adds one straight line to what the next Flush draws.
func (p *Painter) AddLine(a, b city.Point, width float64, c color.NRGBA) {
	p.line = [2]city.Point{a, b}
	p.mesh.Stroke(p.line[:], width, false, c)
}

// Flush draws everything added since the last draw.
func (p *Painter) Flush(dst *ebiten.Image) {
	p.flush(dst, ebiten.BlendSourceOver)
}

// Line strokes one straight line width wide.
func (p *Painter) Line(dst *ebiten.Image, x0, y0, x1, y1, width float64, c color.NRGBA) {
	p.line = [2]city.Point{{X: x0, Y: y0}, {X: x1, Y: y1}}
	p.Stroke(dst, p.line[:], width, false, c)
}

// Circle strokes a circle of radius r round centre, width wide.
func (p *Painter) Circle(dst *ebiten.Image, centre city.Point, r, width float64, c color.NRGBA) {
	if !(r > 0) {
		return
	}
	pts := AppendEllipse(p.Points(), centre, r, r)
	p.Stroke(dst, pts, width, true, c)
	p.Keep(pts)
}

// Disc fills a disc of radius r round centre from a stamp.
func (p *Painter) Disc(dst *ebiten.Image, centre city.Point, r float64, c color.NRGBA) {
	p.DiscBlend(dst, centre, r, c, ebiten.BlendSourceOver)
}

// DiscBlend fills a disc with a blend, such as a glow's BlendLighter.
func (p *Painter) DiscBlend(dst *ebiten.Image, centre city.Point, r float64, c color.NRGBA, blend ebiten.Blend) {
	level, scale, ok := discStamp(r)
	if !ok || c.A == 0 {
		return
	}
	stamp := p.disc(level)
	half := float64(stamp.Bounds().Dx()) / 2
	p.img = ebiten.DrawImageOptions{Filter: ebiten.FilterLinear, DisableMipmaps: true, Blend: blend}
	p.img.GeoM.Translate(-half, -half)
	p.img.GeoM.Scale(scale, scale)
	p.img.GeoM.Translate(centre.X, centre.Y)
	p.img.ColorScale.ScaleWithColor(c)
	dst.DrawImage(stamp, &p.img)
}

func (p *Painter) flush(dst *ebiten.Image, blend ebiten.Blend) {
	if len(p.mesh.Indices) > 0 {
		p.tri = ebiten.DrawTrianglesOptions{Blend: blend}
		dst.DrawTriangles32(p.mesh.Vertices, p.mesh.Indices, p.whitePixel(), &p.tri)
	}
	p.mesh.Reset()
}

// fallback fills a ring that crosses itself by the non-zero rule, without
// anti-aliasing, so it costs one pass; the mesh's fringe still smooths
// its edge.
func (p *Painter) fallback(dst *ebiten.Image, pts []city.Point, c color.NRGBA, blend ebiten.Blend) {
	if len(pts) < 3 {
		return
	}
	p.path.Reset()
	for i, pt := range pts {
		if i == 0 {
			p.path.MoveTo(float32(pt.X), float32(pt.Y))
		} else {
			p.path.LineTo(float32(pt.X), float32(pt.Y))
		}
	}
	p.path.Close()
	op := &vector.DrawPathOptions{Blend: blend}
	op.ColorScale.ScaleWithColor(c)
	vector.FillPath(dst, &p.path, &vector.FillOptions{FillRule: vector.FillRuleNonZero}, op)
}

func (p *Painter) whitePixel() *ebiten.Image {
	if p.white == nil {
		img := ebiten.NewImage(3, 3)
		img.Fill(color.White)
		p.white = img.SubImage(image.Rect(1, 1, 2, 2)).(*ebiten.Image)
	}
	return p.white
}

// disc is the stamp for one of discRadii, drawn the first time it is
// asked for: a white disc, anti-aliased once, with two clear pixels
// round it so linear filtering never reads past its edge.
func (p *Painter) disc(level int) *ebiten.Image {
	if p.discs == nil {
		p.discs = make([]*ebiten.Image, len(discRadii))
	}
	if p.discs[level] == nil {
		r := discRadii[level]
		size := int(math.Ceil(2*r)) + 4
		img := ebiten.NewImage(size, size)
		half := float32(size) / 2
		vector.FillCircle(img, half, half, float32(r), color.White, true)
		p.discs[level] = img
	}
	return p.discs[level]
}
