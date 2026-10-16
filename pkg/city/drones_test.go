package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 89: a drone is one subagent in flight. It takes off from a pad on
// its session's roof, loiters, flies to the district its subagent works
// in and back, and lands again when the subagent finishes.
func TestDroneFlights(t *testing.T) {
	start := time.Date(2026, 10, 16, 15, 0, 0, 0, time.UTC)
	flying := func(dirs ...string) state.Snapshot {
		a := session("a", cinders, state.Working)
		a.SubagentsInFlight, a.SubagentDirs = len(dirs), dirs
		return snapshot(a, session("b", botropolis, state.Working))
	}
	// scene starts with no subagent in flight, then shows dirs.
	scene := func(t *testing.T, clock *time.Time, level city.Liveliness, dirs ...string) *city.Scene {
		t.Helper()
		*clock = start
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		s.SetClock(func() time.Time { return *clock }, time.UTC)
		s.SetReducedMotion(level.CrossFades())
		s.SetSnapshot(flying())
		s.SetSnapshot(flying(dirs...))
		return s
	}
	flights := func(s *city.Scene) []city.Flight { return s.Flights("a") }
	home := func(s *city.Scene) city.Point {
		for _, b := range s.City().Buildings() {
			if b.Session.ID == "a" {
				return b.Rect.Center()
			}
		}
		return city.Point{}
	}
	elsewhere := func(s *city.Scene) city.Point {
		for _, d := range s.City().Districts {
			if d.Root == botropolis {
				return d.Rect.Center()
			}
		}
		return city.Point{}
	}

	t.Run("when a subagent starts", func(t *testing.T) {
		var clock time.Time
		s := scene(t, &clock, city.Lively, cinders)
		got := flights(s)

		t.Run("it should put up one drone, taking off", func(t *testing.T) {
			require.Len(t, got, 1)
			assert.Equal(t, city.TakingOff, got[0].Stage)
		})

		t.Run("and TakeoffFor passes", func(t *testing.T) {
			clock = start.Add(city.TakeoffFor)

			t.Run("it should be loitering", func(t *testing.T) {
				assert.Equal(t, city.Loitering, flights(s)[0].Stage)
			})
		})
	})

	t.Run("when two subagents start", func(t *testing.T) {
		var clock time.Time
		got := flights(scene(t, &clock, city.Lively, cinders, cinders))

		t.Run("it should give them different seats", func(t *testing.T) {
			require.Len(t, got, 2)
			assert.NotEqual(t, got[0].Seat, got[1].Seat)
		})
	})

	t.Run("when a subagent finishes", func(t *testing.T) {
		var clock time.Time
		s := scene(t, &clock, city.Lively, cinders)
		clock = start.Add(time.Minute)
		s.SetSnapshot(flying())

		t.Run("it should land its drone", func(t *testing.T) {
			require.Len(t, flights(s), 1)
			assert.Equal(t, city.Landing, flights(s)[0].Stage)
		})

		t.Run("and LandFor passes", func(t *testing.T) {
			clock = start.Add(time.Minute + city.LandFor)

			t.Run("it should be gone", func(t *testing.T) {
				assert.Empty(t, flights(s))
			})
		})
	})

	t.Run("when a drone's subagent finishes while it is still taking off", func(t *testing.T) {
		var clock time.Time
		s := scene(t, &clock, city.Lively, cinders)
		clock = start.Add(city.TakeoffFor / 2)
		s.SetSnapshot(flying())

		t.Run("it should begin landing at once", func(t *testing.T) {
			require.Len(t, flights(s), 1)
			assert.Equal(t, city.Landing, flights(s)[0].Stage)
		})
	})

	t.Run("when a subagent works in another district", func(t *testing.T) {
		var clock time.Time
		s := scene(t, &clock, city.Lively, botropolis+"/pkg/city")
		clock = start.Add(city.TakeoffFor + city.FlyFor)

		t.Run("it should loiter over that district", func(t *testing.T) {
			got := flights(s)
			require.Len(t, got, 1)
			assert.Equal(t, [2]any{city.Loitering, elsewhere(s)}, [2]any{got[0].Stage, got[0].Over})
		})

		t.Run("and it finishes", func(t *testing.T) {
			clock = start.Add(time.Minute)
			s.SetSnapshot(flying())
			clock = clock.Add(city.FlyFor + city.LandFor - time.Millisecond)

			t.Run("it should land on its own pad", func(t *testing.T) {
				got := flights(s)
				require.Len(t, got, 1)
				assert.Equal(t, [2]any{city.Landing, home(s)}, [2]any{got[0].Stage, got[0].Over})
			})
		})
	})

	t.Run("while a drone takes off", func(t *testing.T) {
		var clock time.Time
		s := scene(t, &clock, city.Lively, cinders)
		clock = start.Add(city.TakeoffFor / 2)

		t.Run("it should keep the scene animating", func(t *testing.T) {
			assert.True(t, s.Animating())
		})
	})

	t.Run("while a drone lands", func(t *testing.T) {
		var clock time.Time
		s := scene(t, &clock, city.Lively, cinders)
		clock = start.Add(time.Minute)
		s.SetSnapshot(flying())
		clock = clock.Add(city.LandFor / 2)

		t.Run("it should keep the scene animating", func(t *testing.T) {
			assert.True(t, s.Animating())
		})
	})

	t.Run("when the level is still", func(t *testing.T) {
		t.Run("and a subagent starts", func(t *testing.T) {
			var clock time.Time
			s := scene(t, &clock, city.Still, cinders)
			clock = start.Add(city.DroneFadeFor / 2)
			got := flights(s)
			require.Len(t, got, 1)

			t.Run("it should fade the drone in over the short fade", func(t *testing.T) {
				assert.InDelta(t, 0.5, got[0].Fade, 1e-9)
			})

			t.Run("it should not climb", func(t *testing.T) {
				assert.Equal(t, 1.0, got[0].Climb)
			})
		})

		t.Run("and a subagent finishes", func(t *testing.T) {
			var clock time.Time
			s := scene(t, &clock, city.Still, cinders)
			clock = start.Add(time.Minute)
			s.SetSnapshot(flying())
			clock = clock.Add(city.DroneFadeFor / 2)

			t.Run("it should fade the drone out over the short fade", func(t *testing.T) {
				require.Len(t, flights(s), 1)
				assert.InDelta(t, 0.5, flights(s)[0].Fade, 1e-9)
			})
		})

		t.Run("and its subagent works in another district", func(t *testing.T) {
			var clock time.Time
			s := scene(t, &clock, city.Still, botropolis+"/pkg/city")
			clock = start.Add(city.DroneFadeFor / 2)

			t.Run("it should stand over that district as a marker, with no flight", func(t *testing.T) {
				require.Len(t, flights(s), 1)
				assert.Equal(t, elsewhere(s), flights(s)[0].Over)
			})
		})
	})
}
