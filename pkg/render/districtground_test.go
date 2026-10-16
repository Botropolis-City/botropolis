package render

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

// Item 86: when every session in a district has reached a rung, the
// district's whole floor shows it, edge to edge.
func TestDistrictGround(t *testing.T) {
	at := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	district := func(t *testing.T, quiet time.Duration, live bool) *city.District {
		t.Helper()
		sessions := []state.Session{
			{ID: "a", CWD: "/src/kiln", State: state.Resting, LastActivity: at.Add(-quiet)},
			{ID: "b", CWD: "/src/kiln", State: state.Resting, LastActivity: at.Add(-quiet - day)},
			{ID: "c", CWD: "/src/kiln", State: state.Resting, LastActivity: at.Add(-quiet - 2*day)},
		}
		if live {
			sessions[2].State = state.Working
		}
		c := city.Build(state.Snapshot{At: at, Sessions: sessions}, city.NewLayout())
		require.Len(t, c.Districts, 1)
		return c.Districts[0]
	}

	t.Run("when the district is live", func(t *testing.T) {
		g := dressDistrict(district(t, 40*day, true))

		t.Run("its ground should not age", func(t *testing.T) {
			assert.Equal(t, districtGround{}, g)
		})
	})

	for _, c := range []struct {
		rung                         string
		quiet                        time.Duration
		worn, cracked, mossy, growth bool
	}{
		{"resting", day, true, false, false, false},
		{"shuttered", 10 * day, true, true, false, false},
		{"overgrown", 40 * day, true, true, true, true},
	} {
		t.Run("when every session in the district is "+c.rung, func(t *testing.T) {
			g := dressDistrict(district(t, c.quiet, false))

			t.Run("its whole floor should wear", func(t *testing.T) {
				assert.Equal(t, c.worn, g.Wear > 0)
			})

			t.Run("its floor should crack only once shuttered", func(t *testing.T) {
				assert.Equal(t, c.cracked, len(g.Cracks) > 0)
			})

			t.Run("its floor should turn mossy only once overgrown", func(t *testing.T) {
				assert.Equal(t, c.mossy, g.Mossy)
			})

			t.Run("it should grow across the floor only once overgrown", func(t *testing.T) {
				assert.Equal(t, c.growth, len(g.Weeds) > 0)
			})
		})
	}

	t.Run("when the district is overgrown", func(t *testing.T) {
		d := district(t, 40*day, false)
		g := dressDistrict(d)
		left, right, top, bottom := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
		for _, w := range g.Weeds {
			left, right = math.Min(left, w.At.X), math.Max(right, w.At.X)
			top, bottom = math.Min(top, w.At.Y), math.Max(bottom, w.At.Y)
		}

		t.Run("it should grow across the floor edge to edge", func(t *testing.T) {
			assert.Greater(t, right-left, 0.8*d.Rect.Width())
			assert.Greater(t, bottom-top, 0.8*d.Rect.Height())
		})

		t.Run("it should grow the same way every time", func(t *testing.T) {
			assert.Equal(t, g.Weeds, dressDistrict(d).Weeds)
		})

		t.Run("it should grow nothing on a building's footprint", func(t *testing.T) {
			for _, w := range g.Weeds {
				for _, b := range d.Buildings {
					assert.False(t, shrink(b.Rect, 0.1).Contains(w.At), w.At)
				}
			}
		})

		t.Run("it should keep its growth on the district's floor", func(t *testing.T) {
			for _, w := range g.Weeds {
				assert.True(t, d.Rect.Contains(w.At), w.At)
			}
		})
	})
}
