package city_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 86: the depot fits its crates, a row per district, rather than
// spanning the city.
func TestDepotSize(t *testing.T) {
	crate := func(id, cwd string) state.Session {
		s := session(id, cwd, state.Resting)
		s.Packed = true
		return s
	}
	crates := func(cwd string, n int) []state.Session {
		var out []state.Session
		for i := 0; i < n; i++ {
			out = append(out, crate(fmt.Sprintf("%s-%d", cwd, i), cwd))
		}
		return out
	}
	depotIn := func(c *city.City) *city.District {
		for _, d := range c.Districts {
			if d.Depot {
				return d
			}
		}
		return nil
	}
	cells := func(px float64) float64 { return math.Ceil(px/city.CellSize) * city.CellSize }
	pitch := city.CrateSize + city.CrateGap
	row := func(n int) float64 { return 2*city.DistrictPadding + pitch*float64(n) - city.CrateGap }

	t.Run("when one district has one crate", func(t *testing.T) {
		d := depotIn(build(t, city.NewLayout(), append([]state.Session{session("a", cinders, state.Working)}, crates(botropolis, 1)...)...))
		require.NotNil(t, d)

		t.Run("the depot should be one crate wide, in whole cells", func(t *testing.T) {
			assert.Equal(t, cells(row(1)), d.Rect.Width())
		})
	})

	t.Run("when rows differ in length", func(t *testing.T) {
		d := depotIn(build(t, city.NewLayout(), append(append([]state.Session{session("a", cinders, state.Working)}, crates(botropolis, 2)...), crates(mullet, 5)...)...))
		require.NotNil(t, d)

		t.Run("it should be as wide as the longest row", func(t *testing.T) {
			assert.Equal(t, cells(row(5)), d.Rect.Width())
		})

		t.Run("it should be as deep as its rows", func(t *testing.T) {
			assert.Equal(t, cells(2*city.DistrictPadding+2*city.CrateSize+city.RowGap), d.Rect.Height())
		})
	})

	t.Run("when a row is longer than the plan is wide", func(t *testing.T) {
		c := build(t, city.NewLayout(), append([]state.Session{session("a", cinders, state.Working)}, crates(botropolis, 60)...)...)
		d := depotIn(c)
		require.NotNil(t, d)

		t.Run("it should be no wider than the plan", func(t *testing.T) {
			assert.LessOrEqual(t, d.Rect.Width(), float64(c.Plan().SpanCols())*city.CellSize)
		})

		t.Run("it should wrap its crates into more rows", func(t *testing.T) {
			assert.Greater(t, d.Rect.Height(), cells(2*city.DistrictPadding+city.CrateSize))
		})
	})

	t.Run("when nothing is packed up", func(t *testing.T) {
		c := build(t, city.NewLayout(), session("a", cinders, state.Working))

		t.Run("there should be no depot", func(t *testing.T) {
			assert.Nil(t, depotIn(c))
		})
	})

	t.Run("when the last crate is unpacked", func(t *testing.T) {
		at := now
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return at }, time.UTC)
		s.Resize(800, 600)
		packed := crate("b", botropolis)
		s.SetSnapshot(snapshot(session("a", cinders, state.Working), packed))
		packed.Packed = false
		s.SetSnapshot(snapshot(session("a", cinders, state.Working), packed))

		t.Run("the depot should stay while its truck leaves", func(t *testing.T) {
			assert.NotNil(t, depotIn(s.City()))
		})

		t.Run("and the truck has left", func(t *testing.T) {
			at = now.Add(city.PackMax)
			s.Animate(0.016)

			t.Run("the depot should go", func(t *testing.T) {
				assert.Nil(t, depotIn(s.City()))
			})
		})
	})
}
