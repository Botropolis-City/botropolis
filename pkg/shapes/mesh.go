// Package shapes draws the city's lines, quads, rings and discs as small
// meshes instead of anti-aliased vector paths (item 191).
//
// Ebitengine's anti-aliased fill draws each path several times through
// offscreen stencil images, so one small circle becomes thousands of
// indices; on the metropolis demo that was most of the indices a frame
// and most of the heap (item 156). Here a shape is one mesh drawn from a
// white pixel: its core at the shape's alpha, and a one-pixel fringe whose
// outer vertices are transparent, so the edge stays smooth (the technique
// NanoVG uses). A fill's fringe lies wholly outside it, so two fills of
// one colour that share an edge leave no seam; a stroke's straddles its
// edges, so its width stays exact.
package shapes

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// miterLimit is how far a joint's vertex may sit from the joint, in
// multiples of its offset, before the joint is bevelled instead.
const miterLimit = 2.0

// whiteSrc is where every vertex samples the white pixel.
const whiteSrc = 1.5

// Mesh is the vertices and indices of the shapes built into it since its
// last Reset, in buffers it keeps, so building a frame's shapes allocates
// nothing once the buffers have grown.
type Mesh struct {
	Vertices []ebiten.Vertex
	Indices  []uint32

	pts    []city.Point
	norms  []city.Point
	starts []uint32
	ends   []uint32
	rows   []row
	tri    []uint32
	ear    []int
}

type row struct {
	d float64
	a float32
}

// Reset empties the mesh and keeps its buffers.
func (m *Mesh) Reset() {
	m.Vertices = m.Vertices[:0]
	m.Indices = m.Indices[:0]
}

// Fill adds a closed ring of points, filled. It reports false when the
// ring crosses itself and so has no core here, only its fringe; the
// caller fills the core some other way.
func (m *Mesh) Fill(pts []city.Point, c color.NRGBA) bool {
	pts = m.clean(pts, true)
	if len(pts) < 3 {
		return true
	}
	var ok bool
	m.tri, ok = m.triangulate(m.tri[:0], pts)
	m.fill(pts, m.tri, ok, c)
	return ok
}

// Ring remembers a ring's triangulation between frames, for a large ring
// that only moves, such as the City Plan's coast. Triangulation does not
// change when the ring is panned or zoomed, so the caller forgets it only
// when the ring's own shape changes.
type Ring struct {
	tri            []uint32
	n              int
	ok             bool
	known          bool
	triangulations int
}

// Forget drops the remembered triangulation.
func (r *Ring) Forget() { r.known = false }

// FillRing is Fill, triangulating the ring only when the ring has a
// different number of points than last time or was forgotten.
func (m *Mesh) FillRing(pts []city.Point, c color.NRGBA, r *Ring) bool {
	pts = m.clean(pts, true)
	if len(pts) < 3 {
		return true
	}
	if !r.known || r.n != len(pts) {
		r.tri, r.ok = m.triangulate(r.tri[:0], pts)
		r.n, r.known = len(pts), true
		r.triangulations++
	}
	m.fill(pts, r.tri, r.ok, c)
	return r.ok
}

func (m *Mesh) fill(pts []city.Point, tri []uint32, ok bool, c color.NRGBA) {
	a := alpha(c)
	if ok {
		base := uint32(len(m.Vertices))
		for _, p := range pts {
			m.vertex(p, c, a)
		}
		for _, i := range tri {
			m.Indices = append(m.Indices, base+i)
		}
	}
	out := 1.0
	if signedArea(pts) > 0 {
		out = -1
	}
	m.rows = append(m.rows[:0], row{0, a}, row{out, 0})
	m.band(pts, true, c)
}

// Stroke adds a polyline of points, stroked width wide, with butt ends;
// closed joins its last point to its first.
func (m *Mesh) Stroke(pts []city.Point, width float64, closed bool, c color.NRGBA) {
	pts = m.clean(pts, closed)
	if len(pts) < 2 || width <= 0 {
		return
	}
	a := alpha(c)
	if width >= 1 {
		half := width / 2
		m.rows = append(m.rows[:0], row{-half - 0.5, 0}, row{-half + 0.5, a}, row{half - 0.5, a}, row{half + 0.5, 0})
	} else {
		m.rows = append(m.rows[:0], row{-1, 0}, row{0, a * float32(width)}, row{1, 0})
	}
	m.band(pts, closed && len(pts) > 2, c)
}

// band lays the mesh's rows along the polyline: each row is the line
// offset sideways by its distance, at its alpha, and each pair of
// neighbouring rows is filled between. A joint is mitred, or bevelled
// when the mitre would reach past miterLimit.
func (m *Mesh) band(pts []city.Point, closed bool, c color.NRGBA) {
	n := len(pts)
	segs := n - 1
	if closed {
		segs = n
	}
	m.norms = m.norms[:0]
	for i := 0; i < segs; i++ {
		p, q := pts[i], pts[(i+1)%n]
		l := math.Hypot(q.X-p.X, q.Y-p.Y)
		m.norms = append(m.norms, city.Point{X: -(q.Y - p.Y) / l, Y: (q.X - p.X) / l})
	}
	m.starts, m.ends = m.starts[:0], m.ends[:0]
	rows := len(m.rows)
	for i, p := range pts {
		var in, out city.Point
		hasIn, hasOut := closed || i > 0, closed || i < n-1
		if hasIn {
			in = m.norms[(i-1+segs)%segs]
		}
		if hasOut {
			out = m.norms[i%segs]
		}
		switch {
		case !hasIn:
			m.starts = append(m.starts, m.offsets(p, out, 1, c))
			m.ends = append(m.ends, 0)
		case !hasOut:
			m.ends = append(m.ends, m.offsets(p, in, 1, c))
			m.starts = append(m.starts, 0)
		default:
			sum := city.Point{X: in.X + out.X, Y: in.Y + out.Y}
			l := math.Hypot(sum.X, sum.Y)
			var scale float64
			if l > 1e-9 {
				sum = city.Point{X: sum.X / l, Y: sum.Y / l}
				scale = 1 / (sum.X*in.X + sum.Y*in.Y)
			}
			if l > 1e-9 && scale <= miterLimit {
				at := m.offsets(p, sum, scale, c)
				m.ends = append(m.ends, at)
				m.starts = append(m.starts, at)
				continue
			}
			end := m.offsets(p, in, 1, c)
			start := m.offsets(p, out, 1, c)
			m.ends = append(m.ends, end)
			m.starts = append(m.starts, start)
			for r := 0; r+1 < rows; r++ {
				m.quad(end+uint32(r), start+uint32(r), start+uint32(r+1), end+uint32(r+1))
			}
		}
	}
	for i := 0; i < segs; i++ {
		s, e := m.starts[i], m.ends[(i+1)%n]
		for r := 0; r+1 < rows; r++ {
			m.quad(s+uint32(r), e+uint32(r), e+uint32(r+1), s+uint32(r+1))
		}
	}
}

// offsets adds one vertex a row at p, each moved along dir by its row's
// distance times scale, and returns the first's index.
func (m *Mesh) offsets(p, dir city.Point, scale float64, c color.NRGBA) uint32 {
	first := uint32(len(m.Vertices))
	for _, r := range m.rows {
		m.vertex(city.Point{X: p.X + dir.X*r.d*scale, Y: p.Y + dir.Y*r.d*scale}, c, r.a)
	}
	return first
}

func (m *Mesh) quad(a, b, c, d uint32) {
	m.Indices = append(m.Indices, a, b, c, a, c, d)
}

func (m *Mesh) vertex(p city.Point, c color.NRGBA, a float32) {
	m.Vertices = append(m.Vertices, ebiten.Vertex{
		DstX: float32(p.X), DstY: float32(p.Y),
		SrcX: whiteSrc, SrcY: whiteSrc,
		ColorR: float32(c.R) / 255, ColorG: float32(c.G) / 255, ColorB: float32(c.B) / 255,
		ColorA: a,
	})
}

func alpha(c color.NRGBA) float32 { return float32(c.A) / 255 }

// clean drops repeated points, and a closed ring's last point when it
// repeats its first, into the mesh's own buffer.
func (m *Mesh) clean(pts []city.Point, closed bool) []city.Point {
	m.pts = m.pts[:0]
	for _, p := range pts {
		if k := len(m.pts); k > 0 && same(m.pts[k-1], p) {
			continue
		}
		m.pts = append(m.pts, p)
	}
	for closed && len(m.pts) > 1 && same(m.pts[0], m.pts[len(m.pts)-1]) {
		m.pts = m.pts[:len(m.pts)-1]
	}
	return m.pts
}

func same(p, q city.Point) bool {
	return math.Abs(p.X-q.X) < 1e-9 && math.Abs(p.Y-q.Y) < 1e-9
}

func signedArea(pts []city.Point) float64 {
	a := 0.0
	for i, p := range pts {
		q := pts[(i+1)%len(pts)]
		a += p.X*q.Y - q.X*p.Y
	}
	return a / 2
}
