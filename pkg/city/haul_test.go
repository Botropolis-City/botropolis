package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 86, slice 3: a truck carries a packed-up session's crate from its
// lot to the depot, and back when it is unpacked.
func TestHauls(t *testing.T) {
	lot := session("b", cinders, state.Resting)
	crate := lot
	crate.Packed = true
	at := now
	haul := func(t *testing.T, from, to state.Session, reduced bool) (*city.Scene, city.Point, city.Point) {
		t.Helper()
		at = now
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return at }, time.UTC)
		s.SetReducedMotion(reduced)
		s.Resize(800, 600)
		s.SetSnapshot(snapshot(session("a", cinders, state.Working), from))
		start := centreIn(t, s, "b")
		s.SetSnapshot(snapshot(session("a", cinders, state.Working), to))
		return s, start, centreIn(t, s, "b")
	}

	t.Run("when a session is packed up", func(t *testing.T) {
		s, from, to := haul(t, lot, crate, false)
		hauls := s.Hauls()
		require.Len(t, hauls, 1)

		t.Run("it should send a truck to the depot", func(t *testing.T) {
			assert.Equal(t, city.HaulPack, hauls[0].Kind)
		})

		t.Run("it should start at the session's lot", func(t *testing.T) {
			assert.Equal(t, from, hauls[0].At(now))
		})

		t.Run("it should hide the crate in the depot until the truck arrives", func(t *testing.T) {
			assert.True(t, s.Hauling("b"))
		})

		t.Run("and the clock is advanced by half of PackFor", func(t *testing.T) {
			mid := hauls[0].At(now.Add(city.PackFor / 2))

			t.Run("it should be on its way, between the lot and the depot", func(t *testing.T) {
				assert.True(t, mid.Y > from.Y && mid.Y < to.Y, "%v not between %v and %v", mid, from, to)
			})

			t.Run("it should ease out, past halfway at half the time", func(t *testing.T) {
				assert.Greater(t, mid.Y, (from.Y+to.Y)/2)
			})
		})

		t.Run("and the clock is advanced by PackFor", func(t *testing.T) {
			at = now.Add(city.PackFor)

			t.Run("it should have delivered the crate", func(t *testing.T) {
				assert.Empty(t, s.Hauls())
			})
		})
	})

	t.Run("when a crate is unpacked", func(t *testing.T) {
		s, from, to := haul(t, crate, lot, false)
		hauls := s.Hauls()
		require.Len(t, hauls, 1)

		t.Run("it should send a truck back", func(t *testing.T) {
			assert.Equal(t, city.HaulUnpack, hauls[0].Kind)
		})

		t.Run("it should start at the depot", func(t *testing.T) {
			assert.Equal(t, from, hauls[0].At(now))
		})

		t.Run("it should end at the old lot", func(t *testing.T) {
			assert.Equal(t, to, hauls[0].At(now.Add(city.PackFor)))
		})
	})

	t.Run("when reduced motion is on", func(t *testing.T) {
		s, _, _ := haul(t, lot, crate, true)
		at = now.Add(500 * time.Millisecond)

		t.Run("it should be a cross-fade done in under half a second", func(t *testing.T) {
			assert.Empty(t, s.Hauls())
		})
	})
}

func centreIn(t *testing.T, s *city.Scene, id string) city.Point {
	t.Helper()
	for _, b := range s.City().Buildings() {
		if b.Session.ID == id {
			return b.Rect.Center()
		}
	}
	require.FailNow(t, "no building for "+id)
	return city.Point{}
}
