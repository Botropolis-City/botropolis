package shapes

import (
	"math"

	"botropolis.city/botropolis/pkg/city"
)

// Triangulate appends to dst the triangles that tile a simple ring, as
// indices into pts, by ear clipping. It reports false, appending nothing,
// for a ring that crosses itself or has no area: those have no tiling of
// their own, and the painter fills them by the non-zero rule instead.
func Triangulate(dst []uint32, pts []city.Point) ([]uint32, bool) {
	var m Mesh
	return m.triangulate(dst, pts)
}

func (m *Mesh) triangulate(dst []uint32, pts []city.Point) ([]uint32, bool) {
	n := len(pts)
	area := signedArea(pts)
	if n < 3 || math.Abs(area) < 1e-12 || crosses(pts) {
		return dst, false
	}
	wind := 1.0
	if area < 0 {
		wind = -1
	}
	m.ear = m.ear[:0]
	for i := range n {
		m.ear = append(m.ear, i)
	}
	mark := len(dst)
	ear := m.ear
	for i, misses := 0, 0; len(ear) > 3; {
		k := len(ear)
		a, b, c := ear[(i+k-1)%k], ear[i%k], ear[(i+1)%k]
		turn := cross(pts[a], pts[b], pts[c]) * wind
		switch {
		case math.Abs(turn) < 1e-12:
			ear = append(ear[:i%k], ear[i%k+1:]...)
			misses = 0
		case turn > 0 && empty(pts, ear, a, b, c):
			dst = append(dst, uint32(a), uint32(b), uint32(c))
			ear = append(ear[:i%k], ear[i%k+1:]...)
			misses = 0
		default:
			i++
			misses++
			if misses > k {
				return dst[:mark], false
			}
			continue
		}
		if len(ear) > 0 {
			i %= len(ear)
		}
	}
	if len(ear) == 3 {
		if math.Abs(cross(pts[ear[0]], pts[ear[1]], pts[ear[2]])) >= 1e-12 {
			dst = append(dst, uint32(ear[0]), uint32(ear[1]), uint32(ear[2]))
		}
	}
	m.ear = ear
	return dst, true
}

func cross(a, b, c city.Point) float64 {
	return (b.X-a.X)*(c.Y-b.Y) - (b.Y-a.Y)*(c.X-b.X)
}

// empty reports whether no other remaining point of the ring lies in the
// triangle a, b, c.
func empty(pts []city.Point, ear []int, a, b, c int) bool {
	pa, pb, pc := pts[a], pts[b], pts[c]
	for _, i := range ear {
		if i == a || i == b || i == c {
			continue
		}
		p := pts[i]
		if same(p, pa) || same(p, pb) || same(p, pc) {
			continue
		}
		d1, d2, d3 := side(p, pa, pb), side(p, pb, pc), side(p, pc, pa)
		neg := d1 < 0 || d2 < 0 || d3 < 0
		pos := d1 > 0 || d2 > 0 || d3 > 0
		if !neg || !pos {
			return false
		}
	}
	return true
}

func side(p, a, b city.Point) float64 {
	return (p.X-b.X)*(a.Y-b.Y) - (a.X-b.X)*(p.Y-b.Y)
}

// crosses reports whether any two edges of the ring that do not share a
// point cross each other.
func crosses(pts []city.Point) bool {
	n := len(pts)
	for i := range n {
		a, b := pts[i], pts[(i+1)%n]
		for j := i + 2; j < n; j++ {
			if i == 0 && j == n-1 {
				continue
			}
			if intersect(a, b, pts[j], pts[(j+1)%n]) {
				return true
			}
		}
	}
	return false
}

func intersect(a, b, c, d city.Point) bool {
	d1, d2 := side(c, a, b), side(d, a, b)
	d3, d4 := side(a, c, d), side(b, c, d)
	return ((d1 > 0 && d2 < 0) || (d1 < 0 && d2 > 0)) && ((d3 > 0 && d4 < 0) || (d3 < 0 && d4 > 0))
}
