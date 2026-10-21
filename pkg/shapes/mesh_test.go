package shapes

import (
	"image/color"
	"math"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
)

var ink = color.NRGBA{R: 40, G: 80, B: 120, A: 200}

// mass is the integral of alpha over a mesh, in pixels of ink at full
// strength, exact per triangle: alpha varies linearly across each one,
// so its integral is the triangle's area times the mean of its corners'
// alphas.
func mass(m *Mesh) float64 {
	return rawMass(m) / (float64(ink.A) / 255)
}

func rawMass(m *Mesh) float64 {
	total := 0.0
	for i := 0; i+2 < len(m.Indices); i += 3 {
		a, b, c := m.Vertices[m.Indices[i]], m.Vertices[m.Indices[i+1]], m.Vertices[m.Indices[i+2]]
		area := math.Abs(float64((b.DstX-a.DstX)*(c.DstY-a.DstY)-(c.DstX-a.DstX)*(b.DstY-a.DstY))) / 2
		total += area * float64(a.ColorA+b.ColorA+c.ColorA) / 3
	}
	return total
}

func outer(m *Mesh) []ebiten.Vertex {
	var out []ebiten.Vertex
	for _, v := range m.Vertices {
		if v.ColorA == 0 {
			out = append(out, v)
		}
	}
	return out
}

func inner(m *Mesh) []ebiten.Vertex {
	var in []ebiten.Vertex
	for _, v := range m.Vertices {
		if v.ColorA != 0 {
			in = append(in, v)
		}
	}
	return in
}

func ringArea(pts []city.Point) float64 {
	a := 0.0
	for i := range pts {
		p, q := pts[i], pts[(i+1)%len(pts)]
		a += p.X*q.Y - q.X*p.Y
	}
	return math.Abs(a) / 2
}

func perimeter(pts []city.Point, closed bool) float64 {
	n := len(pts)
	if !closed {
		n--
	}
	l := 0.0
	for i := 0; i < n; i++ {
		p, q := pts[i], pts[(i+1)%len(pts)]
		l += math.Hypot(q.X-p.X, q.Y-p.Y)
	}
	return l
}

// filled is what an outward-fringed fill should cover: the shape, and
// half a pixel all round it.
func filled(pts []city.Point) float64 {
	return ringArea(pts) + perimeter(pts, true)/2
}

// corners is how much a fill's mitred corners may add to or take from
// filled: a corner turning by φ mitres a one-pixel fringe over
// tan(φ/2) square pixels, at most half of it inked.
func corners(pts []city.Point) float64 {
	slack := 0.0
	for i := range pts {
		a, b, c := pts[(i+len(pts)-1)%len(pts)], pts[i], pts[(i+1)%len(pts)]
		turn := math.Abs(math.Atan2(c.Y-b.Y, c.X-b.X) - math.Atan2(b.Y-a.Y, b.X-a.X))
		if turn > math.Pi {
			turn = 2*math.Pi - turn
		}
		slack += math.Min(math.Tan(turn/2), miterLimit) / 2
	}
	return slack
}

// assertFilled asserts a fill covers the ring and half a pixel around it,
// to within 2% and what its corners account for.
func assertFilled(t *testing.T, ring []city.Point, m *Mesh) {
	t.Helper()
	assert.InDelta(t, filled(ring), mass(m), 0.02*filled(ring)+corners(ring))
}

func inside(pts []city.Point, x, y float64) bool {
	in := false
	for i, j := 0, len(pts)-1; i < len(pts); j, i = i, i+1 {
		a, b := pts[i], pts[j]
		if (a.Y > y) != (b.Y > y) && x < (b.X-a.X)*(y-a.Y)/(b.Y-a.Y)+a.X {
			in = !in
		}
	}
	return in
}

func square(x, y, s float64) []city.Point {
	return []city.Point{{X: x, Y: y}, {X: x + s, Y: y}, {X: x + s, Y: y + s}, {X: x, Y: y + s}}
}

func reversed(pts []city.Point) []city.Point {
	out := make([]city.Point, len(pts))
	for i, p := range pts {
		out[len(pts)-1-i] = p
	}
	return out
}

// halfAnnulus is the shape the plant's stack curb fills: an outer arc
// one way and an inner arc back.
func halfAnnulus() []city.Point {
	var pts []city.Point
	for i := 0; i <= 16; i++ {
		t := math.Pi + math.Pi*float64(i)/16
		pts = append(pts, city.Point{X: 50 + 40*math.Cos(t), Y: 50 + 15*math.Sin(t)})
	}
	for i := 0; i <= 16; i++ {
		t := math.Pi * float64(i) / 16
		pts = append(pts, city.Point{X: 50 + 30*math.Cos(-t), Y: 50 + 10*math.Sin(-t)})
	}
	return pts
}

func TestFill(t *testing.T) {
	t.Run("when a 10×10 square is filled", func(t *testing.T) {
		sq := square(5, 5, 10)
		var m Mesh
		m.Fill(sq, ink)

		t.Run("it should cover the square and half a pixel around its outline", func(t *testing.T) {
			assertFilled(t, sq, &m)
		})

		t.Run("it should make every inner vertex the fill's alpha", func(t *testing.T) {
			for _, v := range inner(&m) {
				require.InDelta(t, 200.0/255, v.ColorA, 1e-6)
			}
		})

		t.Run("it should put every outer vertex outside the square, within a pixel and a half", func(t *testing.T) {
			for _, v := range outer(&m) {
				x, y := float64(v.DstX), float64(v.DstY)
				dx := math.Max(0, math.Max(5-x, x-15))
				dy := math.Max(0, math.Max(5-y, y-15))
				d := math.Hypot(dx, dy)
				require.True(t, d > 0.5 && d <= 1.5, "outer vertex (%v, %v) is %v from the square", x, y, d)
			}
		})

		t.Run("and it is wound the other way", func(t *testing.T) {
			var back Mesh
			back.Fill(reversed(sq), ink)

			t.Run("it should cover the same", func(t *testing.T) {
				assert.InDelta(t, mass(&m), mass(&back), 1e-3)
			})
		})
	})

	concave := []struct {
		name string
		ring []city.Point
	}{
		{"an L-shaped ring", []city.Point{{X: 0, Y: 0}, {X: 20, Y: 0}, {X: 20, Y: 10}, {X: 10, Y: 10}, {X: 10, Y: 20}, {X: 0, Y: 20}}},
		{"a ring with a notch", []city.Point{{X: 0, Y: 0}, {X: 8, Y: 0}, {X: 10, Y: 9}, {X: 12, Y: 0}, {X: 20, Y: 0}, {X: 20, Y: 12}, {X: 0, Y: 12}}},
		{"the stack's half annulus", halfAnnulus()},
	}
	for _, c := range concave {
		t.Run("when "+c.name+" is filled", func(t *testing.T) {
			var m Mesh
			ok := m.Fill(c.ring, ink)

			t.Run("it should triangulate it", func(t *testing.T) {
				assert.True(t, ok)
			})

			t.Run("it should cover it and half a pixel around its outline", func(t *testing.T) {
				assertFilled(t, c.ring, &m)
			})

			t.Run("its core triangles should add up to the ring's own area", func(t *testing.T) {
				tri, ok := Triangulate(nil, c.ring)
				require.True(t, ok)
				sum := 0.0
				for i := 0; i+2 < len(tri); i += 3 {
					sum += ringArea([]city.Point{c.ring[tri[i]], c.ring[tri[i+1]], c.ring[tri[i+2]]})
				}
				assert.InDelta(t, ringArea(c.ring), sum, 1e-6)
			})

			t.Run("it should put no core triangle outside the ring", func(t *testing.T) {
				tri, _ := Triangulate(nil, c.ring)
				for i := 0; i+2 < len(tri); i += 3 {
					a, b, d := c.ring[tri[i]], c.ring[tri[i+1]], c.ring[tri[i+2]]
					require.True(t, inside(c.ring, (a.X+b.X+d.X)/3, (a.Y+b.Y+d.Y)/3), "triangle %v %v %v", a, b, d)
				}
			})
		})
	}

	t.Run("when a polygon is thinner than a pixel", func(t *testing.T) {
		sliver := []city.Point{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 0.4}, {X: 0, Y: 0.4}}
		var m Mesh
		m.Fill(sliver, ink)

		t.Run("it should cover it and half a pixel around its outline", func(t *testing.T) {
			assertFilled(t, sliver, &m)
		})
	})

	t.Run("when a ring crosses itself", func(t *testing.T) {
		bowtie := []city.Point{{X: 0, Y: 0}, {X: 10, Y: 10}, {X: 10, Y: 0}, {X: 0, Y: 10}}
		var m Mesh

		t.Run("it should report it cannot triangulate it, so the painter falls back", func(t *testing.T) {
			assert.False(t, m.Fill(bowtie, ink))
		})
	})

	t.Run("when fewer than three points are filled", func(t *testing.T) {
		var m Mesh
		m.Fill([]city.Point{{X: 0, Y: 0}, {X: 5, Y: 5}}, ink)

		t.Run("it should draw nothing", func(t *testing.T) {
			assert.Empty(t, m.Indices)
		})
	})
}

func TestFillRing(t *testing.T) {
	t.Run("when the same ring is drawn on the next frame", func(t *testing.T) {
		var m Mesh
		var r Ring
		ring := halfAnnulus()
		m.FillRing(ring, ink, &r)
		m.Reset()
		moved := make([]city.Point, len(ring))
		for i, p := range ring {
			moved[i] = city.Point{X: p.X*2 + 7, Y: p.Y*2 - 3}
		}
		m.FillRing(moved, ink, &r)

		t.Run("it should not triangulate it again", func(t *testing.T) {
			assert.Equal(t, 1, r.triangulations)
		})

		t.Run("it should still cover where the ring now is", func(t *testing.T) {
			assertFilled(t, moved, &m)
		})
	})

	t.Run("when a ring with a different number of points is drawn", func(t *testing.T) {
		var m Mesh
		var r Ring
		m.FillRing(halfAnnulus(), ink, &r)
		m.FillRing(square(0, 0, 10), ink, &r)

		t.Run("it should triangulate it afresh", func(t *testing.T) {
			assert.Equal(t, 2, r.triangulations)
		})
	})

	t.Run("when the caller forgets the ring", func(t *testing.T) {
		var m Mesh
		var r Ring
		m.FillRing(square(0, 0, 10), ink, &r)
		r.Forget()
		m.FillRing(square(0, 0, 10), ink, &r)

		t.Run("it should triangulate it afresh", func(t *testing.T) {
			assert.Equal(t, 2, r.triangulations)
		})
	})
}

func TestStroke(t *testing.T) {
	t.Run("when a 10-pixel line is stroked 3 wide", func(t *testing.T) {
		var m Mesh
		m.Stroke([]city.Point{{X: 0, Y: 0}, {X: 10, Y: 0}}, 3, false, ink)

		t.Run("it should cover thirty pixels", func(t *testing.T) {
			assert.InDelta(t, 30, mass(&m), 0.3)
		})

		t.Run("it should fade to transparent half a pixel beyond its edges", func(t *testing.T) {
			require.NotEmpty(t, outer(&m))
			for _, v := range outer(&m) {
				require.InDelta(t, 2, math.Abs(float64(v.DstY)), 1e-4)
			}
		})
	})

	t.Run("when a line is stroked thinner than a pixel", func(t *testing.T) {
		var m Mesh
		m.Stroke([]city.Point{{X: 0, Y: 0}, {X: 10, Y: 0}}, 0.5, false, ink)

		t.Run("it should cover its own area", func(t *testing.T) {
			assert.InDelta(t, 5, mass(&m), 0.05)
		})
	})

	t.Run("when a polyline turns a right angle", func(t *testing.T) {
		var m Mesh
		m.Stroke([]city.Point{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}}, 2, false, ink)

		t.Run("it should cover its length times its width, painting the joint once", func(t *testing.T) {
			assert.InDelta(t, 40, mass(&m), 0.4)
		})
	})

	t.Run("when a polyline turns an acute angle", func(t *testing.T) {
		var m Mesh
		const width = 2.0
		m.Stroke([]city.Point{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 0, Y: 1}}, width, false, ink)

		t.Run("it should reach no further than one width beyond the joint", func(t *testing.T) {
			for _, v := range m.Vertices {
				require.LessOrEqual(t, float64(v.DstX), 10+width)
			}
		})
	})

	t.Run("when a closed ring is stroked", func(t *testing.T) {
		var m Mesh
		m.Stroke(square(0, 0, 10), 2, true, ink)

		t.Run("it should join its last point to its first", func(t *testing.T) {
			assert.InDelta(t, 80, mass(&m), 0.8)
		})
	})

	t.Run("when a polyline repeats a point", func(t *testing.T) {
		var m Mesh
		m.Stroke([]city.Point{{X: 0, Y: 0}, {X: 5, Y: 0}, {X: 5, Y: 0}, {X: 10, Y: 0}}, 2, false, ink)

		t.Run("it should stroke it as if the point were there once", func(t *testing.T) {
			assert.InDelta(t, 20, mass(&m), 0.2)
		})
	})

	t.Run("when a line has no length", func(t *testing.T) {
		var m Mesh
		m.Stroke([]city.Point{{X: 3, Y: 3}, {X: 3, Y: 3}}, 2, false, ink)

		t.Run("it should draw nothing", func(t *testing.T) {
			assert.Empty(t, m.Indices)
		})
	})
}

func TestMeshBatch(t *testing.T) {
	t.Run("when two lines are stroked into one mesh", func(t *testing.T) {
		var m Mesh
		m.Stroke([]city.Point{{X: 0, Y: 0}, {X: 10, Y: 0}}, 2, false, ink)
		m.Stroke([]city.Point{{X: 0, Y: 10}, {X: 10, Y: 10}}, 2, false, ink)

		t.Run("it should cover both", func(t *testing.T) {
			assert.InDelta(t, 40, mass(&m), 0.4)
		})
	})
}

func TestMeshAllocation(t *testing.T) {
	t.Run("when a frame's shapes are built twice into one mesh", func(t *testing.T) {
		var m Mesh
		var r Ring
		ring := halfAnnulus()
		frame := func() {
			m.Reset()
			m.Fill(square(0, 0, 10), ink)
			m.Fill(ring, ink)
			m.FillRing(ring, ink, &r)
			m.Stroke([]city.Point{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}}, 2, false, ink)
			m.Stroke(square(0, 0, 10), 1, true, ink)
		}
		frame()

		t.Run("it should allocate nothing the second time", func(t *testing.T) {
			assert.Zero(t, testing.AllocsPerRun(10, frame))
		})
	})
}
