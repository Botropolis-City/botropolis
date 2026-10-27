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

		span := hauls[0].For

		t.Run("it should send a truck to the depot", func(t *testing.T) {
			assert.Equal(t, city.HaulPack, hauls[0].Kind)
		})

		// Item 304: at the city's truck speed, so a far depot takes longer.
		t.Run("it should take as long as its distance at road speed says", func(t *testing.T) {
			assert.Equal(t, city.HaulFor(from, to, false), span)
		})

		t.Run("it should start at the session's lot", func(t *testing.T) {
			assert.Equal(t, from, hauls[0].At(now))
		})

		t.Run("it should hide the crate in the depot until the truck arrives", func(t *testing.T) {
			assert.True(t, s.Hauling("b"))
		})

		// Item 74: a building on a truck is not on its lot to light.
		t.Run("it should have its lights off", func(t *testing.T) {
			assert.Zero(t, s.Lights("b"))
		})

		t.Run("and the clock is advanced by half its span", func(t *testing.T) {
			mid := hauls[0].At(now.Add(span / 2))

			t.Run("it should be on its way, between the lot and the depot", func(t *testing.T) {
				assert.True(t, mid.Y > from.Y && mid.Y < to.Y, "%v not between %v and %v", mid, from, to)
			})

			t.Run("it should ease out, past halfway at half the time", func(t *testing.T) {
				assert.Greater(t, mid.Y, (from.Y+to.Y)/2)
			})
		})

		t.Run("and the clock is advanced by its span", func(t *testing.T) {
			at = now.Add(span)

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

		t.Run("it should take as long as its distance at road speed says", func(t *testing.T) {
			assert.Equal(t, city.HaulFor(from, to, false), hauls[0].For)
		})

		t.Run("it should end at the old lot", func(t *testing.T) {
			assert.Equal(t, to, hauls[0].At(now.Add(hauls[0].For)))
		})
	})

	// Item 224: two more sessions in cinders re-lay the city mid-haul,
	// which moves the crate's place in the depot.
	t.Run("when a session is packed up, and the depot moves under the crate mid-haul", func(t *testing.T) {
		s, _, was := haul(t, lot, crate, false)
		require.Len(t, s.Hauls(), 1)
		span := s.Hauls()[0].For
		at = now.Add(span / 2)
		here := s.Hauls()[0].At(at)
		s.SetSnapshot(snapshot(session("a", cinders, state.Working), crate, session("c0", cinders, state.Working), session("c1", cinders, state.Working)))
		moved := centreIn(t, s, "b")
		require.NotEqual(t, was, moved, "the fixture should move the crate")
		hauls := s.Hauls()

		t.Run("it should carry on from where it was", func(t *testing.T) {
			require.Len(t, hauls, 1)
			assert.InDelta(t, 0, hauls[0].At(at).DistanceToSegment(here, here), 1e-9)
		})

		t.Run("it should end where the crate now stands", func(t *testing.T) {
			require.Len(t, hauls, 1)
			assert.InDelta(t, 0, hauls[0].At(now.Add(span)).DistanceToSegment(moved, moved), 1e-9)
		})

		t.Run("it should arrive when it would have", func(t *testing.T) {
			at = now.Add(span)
			assert.Empty(t, s.Hauls())
		})
	})

	t.Run("when a crate is unpacked again before its pack truck arrives", func(t *testing.T) {
		s, _, depot := haul(t, lot, crate, false)
		at = now.Add(s.Hauls()[0].For / 2)
		s.SetSnapshot(snapshot(session("a", cinders, state.Working), lot))
		var pack []city.Haul
		for _, h := range s.Hauls() {
			if h.Kind == city.HaulPack {
				pack = append(pack, h)
			}
		}

		t.Run("it should still end the pack truck at the depot", func(t *testing.T) {
			require.Len(t, pack, 1)
			assert.Equal(t, depot, pack[0].To)
		})
	})

	t.Run("when reduced motion is on", func(t *testing.T) {
		s, _, _ := haul(t, lot, crate, true)

		t.Run("and the cross-fade is half way", func(t *testing.T) {
			at = now.Add(city.PackFadeFor / 2)

			t.Run("it should have its lights half on", func(t *testing.T) {
				assert.InDelta(t, 0.5, s.Lights("b"), 1e-9)
			})
		})

		t.Run("and half a second has passed", func(t *testing.T) {
			at = now.Add(500 * time.Millisecond)

			t.Run("it should be a cross-fade done in under half a second", func(t *testing.T) {
				assert.Empty(t, s.Hauls())
			})
		})
	})
}

// Item 304: a haul drives at the city's truck speed, so a far depot
// takes longer, between PackMin and PackMax (Aria, 2026-10-27).
func TestHaulFor(t *testing.T) {
	lot := city.Point{X: 40, Y: -25}
	away := func(cells float64) city.Point { return lot.Add(city.Point{X: cells * city.CellSize}) }

	t.Run("when a truck drives a haul", func(t *testing.T) {
		for _, c := range []struct {
			name  string
			cells float64
			want  time.Duration
		}{
			{"and the depot is 3 cells away, it should take PackMin", 3, city.PackMin},
			{"and the depot is 12 cells away, it should take the distance at road speed", 12, 4 * time.Second},
			{"and the depot is 30 cells away, it should take PackMax", 30, city.PackMax},
		} {
			t.Run(c.name, func(t *testing.T) {
				assert.InDelta(t, float64(c.want), float64(city.HaulFor(lot, away(c.cells), false)), float64(time.Millisecond))
			})
		}
	})

	t.Run("when motion is reduced, and the depot is 30 cells away", func(t *testing.T) {
		t.Run("it should cross-fade in PackFadeFor", func(t *testing.T) {
			assert.Equal(t, city.PackFadeFor, city.HaulFor(lot, away(30), true))
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
