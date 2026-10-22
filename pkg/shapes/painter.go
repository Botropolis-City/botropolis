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
	// Cut, when set, is told of every shape the painter draws over what
	// is beneath it, as it draws it: the city's night lights are cut by
	// whatever stands in front of them (item 195). A glow, drawn with
	// any blend but source-over, is light rather than cover, and is not
	// told.
	Cut Cutter
	// Record, when set, makes every draw the painter would make onto an
	// image: it draws it, or records it when the image is one it
	// records for (item 193).
	Record Recorder
	// key is what was asked of the mesh drawn next: its shapes' points,
	// in the float64 they were given in, and their widths and colours.
	key   MeshKey
	mesh  Mesh
	white *ebiten.Image
	discs []*ebiten.Image
	cuts  *sheet
	path  vector.Path
	line  [2]city.Point
	pts   []city.Point
	tri   ebiten.DrawTrianglesOptions
	img   ebiten.DrawImageOptions
}

// Cutter is told of each shape a Painter draws, in the order it draws
// them, with each shape's alpha in its vertices, its options or alpha.
type Cutter interface {
	CutMesh(vertices []ebiten.Vertex, indices []uint32)
	CutImage(img *ebiten.Image, op *ebiten.DrawImageOptions)
	CutRing(pts []city.Point, alpha float32)
}

// Recorder makes a Painter's draws for it: onto the image, or into a
// record of what would have been drawn there (item 193).
type Recorder interface {
	Image(dst, img *ebiten.Image, op *ebiten.DrawImageOptions)
	// Mesh is given the key the mesh was built from too: its vertices
	// are float32, which rounds a shape a hair differently as it moves,
	// and what was asked of it does not.
	Mesh(dst *ebiten.Image, vertices []ebiten.Vertex, indices []uint32, src *ebiten.Image, op *ebiten.DrawTrianglesOptions, key MeshKey)
	Ring(dst *ebiten.Image, pts []city.Point, op *vector.DrawPathOptions)
}

// MeshKey is what a mesh was built from: the points of its shapes and,
// for each shape, its kind, width, whether it is closed and its colour.
type MeshKey struct {
	Points []city.Point
	Values []float64
}

// shape notes a shape added to the mesh drawn next.
func (p *Painter) shape(kind float64, pts []city.Point, width float64, closed bool, c color.NRGBA) {
	p.key.Points = append(p.key.Points, pts...)
	shut := 0.0
	if closed {
		shut = 1
	}
	p.key.Values = append(p.key.Values, kind, float64(len(pts)), width, shut, float64(c.R), float64(c.G), float64(c.B), float64(c.A))
}

func (p *Painter) drawImage(dst, img *ebiten.Image, op *ebiten.DrawImageOptions) {
	if p.Record != nil {
		p.Record.Image(dst, img, op)
		return
	}
	dst.DrawImage(img, op)
}

// Box fills a rectangle with hard edges, as vector.FillRect does without
// anti-aliasing: the white pixel scaled, which batches with the next.
func (p *Painter) Box(dst *ebiten.Image, x, y, width, height float32, c color.Color) {
	p.img = ebiten.DrawImageOptions{}
	p.img.GeoM.Scale(float64(width), float64(height))
	p.img.GeoM.Translate(float64(x), float64(y))
	p.drawWhite(dst, c)
}

// BoxStroke strokes a rectangle with hard edges, as vector.StrokeRect
// does without anti-aliasing.
func (p *Painter) BoxStroke(dst *ebiten.Image, x, y, width, height, stroke float32, c color.Color) {
	if stroke <= 0 {
		return
	}
	if stroke >= width || stroke >= height {
		p.Box(dst, x-stroke/2, y-stroke/2, width+stroke, height+stroke, c)
		return
	}
	p.Box(dst, x-stroke/2, y-stroke/2, width+stroke, stroke, c)
	p.Box(dst, x-stroke/2, y+height-stroke/2, width+stroke, stroke, c)
	p.Box(dst, x-stroke/2, y+stroke/2, stroke, height-stroke, c)
	p.Box(dst, x+width-stroke/2, y+stroke/2, stroke, height-stroke, c)
}

// Bar strokes a straight line with hard edges, as vector.StrokeLine
// does without anti-aliasing.
func (p *Painter) Bar(dst *ebiten.Image, x0, y0, x1, y1, width float32, c color.Color) {
	p.img = ebiten.DrawImageOptions{}
	p.img.GeoM.Scale(math.Hypot(float64(x1-x0), float64(y1-y0)), float64(width))
	p.img.GeoM.Translate(0, -float64(width)/2)
	p.img.GeoM.Rotate(math.Atan2(float64(y1-y0), float64(x1-x0)))
	p.img.GeoM.Translate(float64(x0), float64(y0))
	p.drawWhite(dst, c)
}

func (p *Painter) drawWhite(dst *ebiten.Image, c color.Color) {
	p.img.ColorScale.ScaleWithColor(c)
	white := p.whitePixel()
	p.drawImage(dst, white, &p.img)
	if p.Cut != nil {
		p.Cut.CutImage(p.cutSheet().white, &p.img)
	}
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
	p.shape(1, pts, 0, true, c)
	if !p.mesh.Fill(pts, c) {
		p.fallback(dst, pts, c, blend)
	}
	p.flush(dst, blend)
}

// FillRing fills a ring whose triangulation r remembers between frames.
func (p *Painter) FillRing(dst *ebiten.Image, pts []city.Point, c color.NRGBA, r *Ring) {
	p.shape(1, pts, 0, true, c)
	if !p.mesh.FillRing(pts, c, r) {
		p.fallback(dst, pts, c, ebiten.BlendSourceOver)
	}
	p.flush(dst, ebiten.BlendSourceOver)
}

// Stroke strokes a polyline of points width wide; closed joins its last
// point to its first.
func (p *Painter) Stroke(dst *ebiten.Image, pts []city.Point, width float64, closed bool, c color.NRGBA) {
	p.shape(2, pts, width, closed, c)
	p.mesh.Stroke(pts, width, closed, c)
	p.flush(dst, ebiten.BlendSourceOver)
}

// AddStroke adds a stroke to what the next Flush draws, so many strokes
// go down in one draw.
func (p *Painter) AddStroke(pts []city.Point, width float64, closed bool, c color.NRGBA) {
	p.shape(2, pts, width, closed, c)
	p.mesh.Stroke(pts, width, closed, c)
}

// AddLine adds one straight line to what the next Flush draws.
func (p *Painter) AddLine(a, b city.Point, width float64, c color.NRGBA) {
	p.line = [2]city.Point{a, b}
	p.shape(2, p.line[:], width, false, c)
	p.mesh.Stroke(p.line[:], width, false, c)
}

// Flush draws everything added since the last draw.
func (p *Painter) Flush(dst *ebiten.Image) {
	p.flush(dst, ebiten.BlendSourceOver)
}

// Line strokes one straight line width wide.
func (p *Painter) Line(dst *ebiten.Image, x0, y0, x1, y1, width float64, c color.NRGBA) {
	p.line = [2]city.Point{{X: x0, Y: y0}, {X: x1, Y: y1}}
	cut := p.Cut
	p.Cut = nil
	p.Stroke(dst, p.line[:], width, false, c)
	p.Cut = cut
	if cut == nil {
		return
	}
	// A line is cut as a hard-edged bar a pixel wider and longer, over
	// its soft edge: a third of its mesh's indices, and the city strokes a
	// scaffold's every tube and board as one (item 195).
	p.img = ebiten.DrawImageOptions{}
	p.img.GeoM.Scale(math.Hypot(x1-x0, y1-y0)+1, width+1)
	p.img.GeoM.Translate(-0.5, -(width+1)/2)
	p.img.GeoM.Rotate(math.Atan2(y1-y0, x1-x0))
	p.img.GeoM.Translate(x0, y0)
	p.img.ColorScale.ScaleAlpha(float32(c.A) / 255)
	cut.CutImage(p.cutSheet().white, &p.img)
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
	p.drawImage(dst, stamp, &p.img)
	if p.Cut != nil && blend == ebiten.BlendSourceOver {
		p.Cut.CutImage(p.cutSheet().discs[level], &p.img)
	}
}

func (p *Painter) flush(dst *ebiten.Image, blend ebiten.Blend) {
	if len(p.mesh.Indices) > 0 {
		p.tri = ebiten.DrawTrianglesOptions{Blend: blend}
		if p.Record != nil {
			p.Record.Mesh(dst, p.mesh.Vertices, p.mesh.Indices, p.whitePixel(), &p.tri, p.key)
		} else {
			dst.DrawTriangles32(p.mesh.Vertices, p.mesh.Indices, p.whitePixel(), &p.tri)
		}
		if p.Cut != nil && blend == ebiten.BlendSourceOver {
			p.Cut.CutMesh(p.mesh.Vertices, p.mesh.Indices)
		}
	}
	p.mesh.Reset()
	p.key.Points, p.key.Values = p.key.Points[:0], p.key.Values[:0]
}

// fallback fills a ring that crosses itself by the non-zero rule, without
// anti-aliasing, so it costs one pass; the mesh's fringe still smooths
// its edge.
func (p *Painter) fallback(dst *ebiten.Image, pts []city.Point, c color.NRGBA, blend ebiten.Blend) {
	if len(pts) < 3 {
		return
	}
	op := &vector.DrawPathOptions{Blend: blend}
	op.ColorScale.ScaleWithColor(c)
	if p.Record != nil {
		p.Record.Ring(dst, pts, op)
	} else {
		p.FillPath(dst, pts, op)
	}
	if p.Cut != nil && blend == ebiten.BlendSourceOver {
		p.Cut.CutRing(pts, float32(c.A)/255)
	}
}

// FillPath fills a ring onto dst by the non-zero rule, without
// anti-aliasing, as the fallback does.
func (p *Painter) FillPath(dst *ebiten.Image, pts []city.Point, op *vector.DrawPathOptions) {
	p.path.Reset()
	for i, pt := range pts {
		if i == 0 {
			p.path.MoveTo(float32(pt.X), float32(pt.Y))
		} else {
			p.path.LineTo(float32(pt.X), float32(pt.Y))
		}
	}
	p.path.Close()
	vector.FillPath(dst, &p.path, &vector.FillOptions{FillRule: vector.FillRuleNonZero}, op)
}

// CutTriangles cuts painted meshes out of dst at their own alpha: how a
// Cutter lays what CutMesh told it.
func (p *Painter) CutTriangles(dst *ebiten.Image, vertices []ebiten.Vertex, indices []uint32) {
	if len(indices) == 0 {
		return
	}
	p.tri = ebiten.DrawTrianglesOptions{Blend: ebiten.BlendDestinationOut}
	dst.DrawTriangles32(vertices, indices, p.cutSheet().white, &p.tri)
}

// CutPath fills a ring onto dst by the non-zero rule, as the fallback
// draws it, cutting it out at alpha: how a Cutter lays a CutRing.
func (p *Painter) CutPath(dst *ebiten.Image, pts []city.Point, alpha float32) {
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
	op := &vector.DrawPathOptions{Blend: ebiten.BlendDestinationOut}
	op.ColorScale.ScaleAlpha(alpha)
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
		size := discSize(level)
		img := ebiten.NewImage(size, size)
		half := float32(size) / 2
		vector.FillCircle(img, half, half, float32(r), color.White, true)
		p.discs[level] = img
	}
	return p.discs[level]
}
