package shapes

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
)

func TestAppendQuad(t *testing.T) {
	t.Run("when a quadratic curve is flattened", func(t *testing.T) {
		a, ctrl, b := city.Point{X: 0, Y: 0}, city.Point{X: 60, Y: 45}, city.Point{X: 120, Y: 0}
		pts := AppendQuad(nil, a, ctrl, b)
		require.GreaterOrEqual(t, len(pts), 3)

		t.Run("it should start at its first point", func(t *testing.T) {
			assert.Equal(t, a, pts[0])
		})

		t.Run("it should end at its last point", func(t *testing.T) {
			assert.Equal(t, b, pts[len(pts)-1])
		})

		t.Run("it should stay within a quarter pixel of the curve", func(t *testing.T) {
			at := func(s float64) city.Point {
				u := 1 - s
				return city.Point{X: u*u*a.X + 2*u*s*ctrl.X + s*s*b.X, Y: u*u*a.Y + 2*u*s*ctrl.Y + s*s*b.Y}
			}
			for i := 0; i <= 1000; i++ {
				p := at(float64(i) / 1000)
				nearest := math.Inf(1)
				for j := 1; j < len(pts); j++ {
					nearest = math.Min(nearest, segmentDistance(p, pts[j-1], pts[j]))
				}
				require.LessOrEqual(t, nearest, 0.25, "curve point %v", p)
			}
		})
	})

	t.Run("when the curve is a straight line", func(t *testing.T) {
		pts := AppendQuad(nil, city.Point{X: 0, Y: 0}, city.Point{X: 5, Y: 0}, city.Point{X: 10, Y: 0})

		t.Run("it should flatten to its two ends", func(t *testing.T) {
			assert.Len(t, pts, 2)
		})
	})
}

func TestAppendEllipse(t *testing.T) {
	t.Run("when a circle is flattened", func(t *testing.T) {
		const r = 30.0
		c := city.Point{X: 50, Y: 50}
		pts := AppendEllipse(nil, c, r, r)

		t.Run("it should stay within a quarter pixel of the circle", func(t *testing.T) {
			for i := range pts {
				mid := city.Point{X: (pts[i].X + pts[(i+1)%len(pts)].X) / 2, Y: (pts[i].Y + pts[(i+1)%len(pts)].Y) / 2}
				require.LessOrEqual(t, r-math.Hypot(mid.X-c.X, mid.Y-c.Y), 0.25)
			}
		})

		t.Run("it should put every point on the circle", func(t *testing.T) {
			for _, p := range pts {
				require.InDelta(t, r, math.Hypot(p.X-c.X, p.Y-c.Y), 1e-9)
			}
		})
	})
}

func TestAppendRoundRect(t *testing.T) {
	t.Run("when a rounded rectangle is flattened", func(t *testing.T) {
		box := city.Rect{Min: city.Point{X: 10, Y: 20}, Max: city.Point{X: 90, Y: 60}}
		pts := AppendRoundRect(nil, box, 8)

		t.Run("it should stay inside its box", func(t *testing.T) {
			for _, p := range pts {
				require.True(t, p.X >= box.Min.X-1e-9 && p.X <= box.Max.X+1e-9 && p.Y >= box.Min.Y-1e-9 && p.Y <= box.Max.Y+1e-9, "%v", p)
			}
		})

		t.Run("it should cover the box less its four rounded corners", func(t *testing.T) {
			want := box.Width()*box.Height() - (4-math.Pi)*8*8
			assert.InDelta(t, want, ringArea(pts), 2*math.Pi*8*tolerance)
		})
	})

	t.Run("when the radius is more than half the box", func(t *testing.T) {
		box := city.Rect{Min: city.Point{X: 0, Y: 0}, Max: city.Point{X: 40, Y: 10}}
		pts := AppendRoundRect(nil, box, 30)

		t.Run("it should round the short side into a semicircle", func(t *testing.T) {
			want := 30*10 + math.Pi*5*5
			assert.InDelta(t, want, ringArea(pts), 2*math.Pi*5*tolerance)
		})
	})
}

func segmentDistance(p, a, b city.Point) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	l := dx*dx + dy*dy
	s := 0.0
	if l > 0 {
		s = math.Max(0, math.Min(1, ((p.X-a.X)*dx+(p.Y-a.Y)*dy)/l))
	}
	return math.Hypot(p.X-(a.X+s*dx), p.Y-(a.Y+s*dy))
}
