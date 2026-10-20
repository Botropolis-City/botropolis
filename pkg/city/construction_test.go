package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 89: a new session's building is built where its tug docks — the
// lot fenced, scaffolding and a crane up, the floors rising, the
// scaffolding down and the lights on.
func TestConstruction(t *testing.T) {
	start := time.Date(2026, 10, 16, 14, 0, 0, 0, time.UTC)
	arriving := func(t *testing.T, clock *time.Time, level city.Liveliness) *city.Scene {
		t.Helper()
		*clock = start
		s := voyageScene(t, clock)
		s.SetReducedMotion(level.CrossFades())
		s.SetSnapshot(snapshot(session("a", cinders, state.Working)))
		*clock = clock.Add(time.Minute)
		s.SetSnapshot(snapshot(session("a", cinders, state.Working), session("b", cinders, state.Working)))
		require.NotEmpty(t, s.Voyages())
		return s
	}
	docked := start.Add(time.Minute + city.SailFor)

	t.Run("when a session's tug is still sailing", func(t *testing.T) {
		var clock time.Time
		s := arriving(t, &clock, city.Lively)
		site, building := s.Construction("b")

		t.Run("it should not have begun building", func(t *testing.T) {
			assert.Equal(t, [2]any{city.Site{}, false}, [2]any{site, building})
		})

		// Item 74: the night glow lit every storey over the empty lot.
		t.Run("it should have its lights off", func(t *testing.T) {
			assert.Zero(t, s.Lights("b"))
		})
	})

	// Item 155: once it docks, a building's lights are on for whatever
	// floors are finished; which floors those are is the renderer's.
	for _, c := range []struct {
		moment string
		p      float64
	}{
		{"just docked", 0},
		{"half built", 0.5},
		{"topped out", 0.75},
		{"scaffolding down", 0.88},
		{"in its last moments", 0.94},
	} {
		t.Run("when the build is "+c.moment, func(t *testing.T) {
			var clock time.Time
			s := arriving(t, &clock, city.Lively)
			clock = docked.Add(time.Duration(c.p * float64(city.BuildFor)))

			t.Run("it should have its lights on for its finished floors", func(t *testing.T) {
				assert.InDelta(t, 1, s.Lights("b"), 1e-6)
			})
		})
	}

	t.Run("when a session has been standing all along", func(t *testing.T) {
		var clock time.Time
		s := arriving(t, &clock, city.Lively)

		t.Run("it should have its lights fully on", func(t *testing.T) {
			assert.InDelta(t, 1, s.Lights("a"), 1e-9)
		})
	})

	for _, c := range []struct {
		moment string
		p      float64
		want   city.Site
	}{
		{"just docked", 0, city.Site{Opacity: 1}},
		{"fenced", 0.1, city.Site{Fence: 1, Opacity: 1}},
		{"scaffolded", 0.25, city.Site{Fence: 1, Scaffold: 1, Crane: 1, Opacity: 1}},
		{"half built", 0.5, city.Site{Fence: 1, Scaffold: 1, Crane: 1, Floors: 0.5, Opacity: 1}},
		{"topped out", 0.75, city.Site{Fence: 1, Scaffold: 1, Crane: 1, Floors: 1, Opacity: 1}},
		{"scaffolding down", 0.88, city.Site{Floors: 1, Opacity: 1}},
		{"lit", 1, city.Site{Floors: 1, Lights: 1, Opacity: 1}},
	} {
		t.Run("when the build is "+c.moment, func(t *testing.T) {
			got := city.SiteAt(c.p)
			for _, part := range []struct {
				name      string
				got, want float64
			}{
				{"fence", got.Fence, c.want.Fence},
				{"scaffolding", got.Scaffold, c.want.Scaffold},
				{"crane", got.Crane, c.want.Crane},
				{"floors", got.Floors, c.want.Floors},
				{"lights", got.Lights, c.want.Lights},
			} {
				t.Run("it should have the "+part.name+" at its place", func(t *testing.T) {
					assert.InDelta(t, part.want, part.got, 1e-9)
				})
			}
		})
	}

	// Item 109: a session already working when its building goes up had
	// its roof scaffold hanging over the empty site at the finished
	// roof's height.
	t.Run("when a working session's building is half built", func(t *testing.T) {
		var clock time.Time
		s := arriving(t, &clock, city.Lively)
		clock = docked.Add(city.BuildFor / 2)
		var b *city.Building
		for _, x := range s.City().Buildings() {
			if x.Session.ID == "b" {
				b = x
			}
		}
		require.NotNil(t, b)

		t.Run("it should put up no roof scaffold until the building stands", func(t *testing.T) {
			assert.Zero(t, s.Scaffold(b))
		})

		t.Run("and the build has finished", func(t *testing.T) {
			clock = docked.Add(city.BuildFor + time.Second)

			t.Run("it should scaffold the roof for the work", func(t *testing.T) {
				assert.Equal(t, 1.0, s.Scaffold(b))
			})
		})
	})

	t.Run("when the build has finished", func(t *testing.T) {
		var clock time.Time
		s := arriving(t, &clock, city.Lively)
		clock = docked.Add(city.BuildFor)
		_, building := s.Construction("b")

		t.Run("it should leave no site", func(t *testing.T) {
			assert.False(t, building)
		})

		t.Run("it should have every floor up", func(t *testing.T) {
			assert.InDelta(t, 1, s.Rising("b"), 1e-9)
		})

		t.Run("it should have its lights fully on", func(t *testing.T) {
			assert.InDelta(t, 1, s.Lights("b"), 1e-9)
		})
	})

	t.Run("BuildFor", func(t *testing.T) {
		// Aria, 2026-10-18, judging true-speed clips: back to the first
		// cut's 1.8 s, which earlier clips had shown about three times fast.
		t.Run("it should be 1.8 seconds", func(t *testing.T) {
			assert.Equal(t, 1800*time.Millisecond, city.BuildFor)
		})
	})

	t.Run("when a newer change arrives mid-build", func(t *testing.T) {
		var clock time.Time
		s := arriving(t, &clock, city.Lively)
		clock = docked.Add(city.BuildFor / 2)
		s.SetSnapshot(snapshot(session("a", cinders, state.Working), session("b", cinders, state.Resting)))
		_, building := s.Construction("b")

		t.Run("it should jump the build to its end", func(t *testing.T) {
			assert.False(t, building)
		})
	})

	t.Run("when the level is still", func(t *testing.T) {
		var clock time.Time
		s := arriving(t, &clock, city.Still)
		var drawn []city.Site
		for _, p := range []float64{0, 0.25, 0.5, 0.75, 0.99} {
			clock = docked.Add(time.Duration(p * float64(city.BuildFadeFor)))
			site, _ := s.Construction("b")
			drawn = append(drawn, site)
		}

		t.Run("it should draw no fence, scaffolding or crane at any moment", func(t *testing.T) {
			var scaffolding float64
			for _, site := range drawn {
				scaffolding += site.Fence + site.Scaffold + site.Crane
			}
			assert.Zero(t, scaffolding)
		})

		t.Run("it should cross-fade the building in over the short fade", func(t *testing.T) {
			assert.InDelta(t, 0.5, drawn[2].Opacity, 1e-9)
		})

		t.Run("and the building is half faded in", func(t *testing.T) {
			clock = docked.Add(city.BuildFadeFor / 2)

			t.Run("it should have its lights half on", func(t *testing.T) {
				assert.InDelta(t, 0.5, s.Lights("b"), 1e-9)
			})
		})
	})

	t.Run("while a building is going up", func(t *testing.T) {
		var clock time.Time
		s := arriving(t, &clock, city.Lively)
		clock = docked.Add(city.BuildFor / 2)

		t.Run("it should be animating", func(t *testing.T) {
			assert.True(t, s.Animating())
		})
	})

	t.Run("while a building is fading in", func(t *testing.T) {
		t.Run("and the level is still", func(t *testing.T) {
			var clock time.Time
			s := arriving(t, &clock, city.Still)
			clock = docked.Add(city.BuildFadeFor / 2)

			t.Run("it should be animating", func(t *testing.T) {
				assert.True(t, s.Animating())
			})
		})
	})
}
