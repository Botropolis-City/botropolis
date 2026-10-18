package city_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 89: the building is the session, so it shows its own work: a
// scaffold on its roof while it is mid-turn, and one load up a hoist for each
// tool call. There is no rover.
func TestBuildingWorks(t *testing.T) {
	start := time.Date(2026, 10, 17, 18, 0, 0, 0, time.UTC)
	with := func(st state.State, calls int) state.Session {
		s := session("a", cinders, st)
		s.SubagentsInFlight, s.ToolCalls = 0, calls
		return s
	}
	type scene struct {
		s     *city.Scene
		clock *time.Time
	}
	open := func(t *testing.T, level city.Liveliness, steps ...state.Session) scene {
		t.Helper()
		clock := start
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		s.SetClock(func() time.Time { return clock }, time.UTC)
		s.SetReducedMotion(level.CrossFades())
		for i, step := range steps {
			clock = start.Add(time.Duration(i) * city.ReadEvery)
			s.SetSnapshot(snapshot(step))
		}
		return scene{s: s, clock: &clock}
	}
	building := func(sc scene) *city.Building {
		for _, b := range sc.s.City().Buildings() {
			if b.Session.ID == "a" {
				return b
			}
		}
		return nil
	}

	t.Run("when a session is mid-turn", func(t *testing.T) {
		sc := open(t, city.Lively, with(state.Working, 0))

		t.Run("its roof should be scaffolded", func(t *testing.T) {
			assert.Equal(t, 1.0, sc.s.Scaffold(building(sc)))
		})
	})

	for _, st := range []state.State{state.NeedsYou, state.Waiting, state.Resting} {
		t.Run("when a session is "+string(st), func(t *testing.T) {
			sc := open(t, city.Lively, with(st, 0))

			t.Run("it should have no scaffold", func(t *testing.T) {
				assert.Zero(t, sc.s.Scaffold(building(sc)))
			})
		})
	}

	// starts is when each load a session's building ran began, watched
	// in steps from the last read until every one has finished.
	starts := func(sc scene, from time.Time) map[time.Duration]bool {
		seen := map[time.Duration]bool{}
		for at := from; at.Before(from.Add(city.ReadEvery + city.LoadFor)); at = at.Add(5 * time.Millisecond) {
			*sc.clock = at
			for _, p := range sc.s.Loads("a") {
				began := at.Sub(from) - time.Duration(p.P*float64(city.LoadFor))
				seen[began.Round(10*time.Millisecond)] = true
			}
		}
		return seen
	}
	merged := func(sc scene, from time.Time) bool {
		for at := from; at.Before(from.Add(city.ReadEvery + city.LoadFor)); at = at.Add(5 * time.Millisecond) {
			*sc.clock = at
			for _, p := range sc.s.Loads("a") {
				if p.Merged {
					return true
				}
			}
		}
		return false
	}
	read := start.Add(city.ReadEvery)

	t.Run("when one tool call is made between reads", func(t *testing.T) {
		sc := open(t, city.Lively, with(state.Working, 0), with(state.Working, 1))

		t.Run("it should send exactly one load", func(t *testing.T) {
			assert.Len(t, starts(sc, read), 1)
		})

		t.Run("and the load has run LoadFor", func(t *testing.T) {
			*sc.clock = read.Add(city.LoadFor)

			t.Run("it should be gone", func(t *testing.T) {
				assert.Empty(t, sc.s.Loads("a"))
			})
		})
	})

	t.Run("when three tool calls are made between reads", func(t *testing.T) {
		sc := open(t, city.Lively, with(state.Working, 0), with(state.Working, 3))

		t.Run("it should send three loads, spread across the interval", func(t *testing.T) {
			assert.Len(t, starts(sc, read), 3)
		})
	})

	t.Run("when more calls are made than fit PulseGap apart", func(t *testing.T) {
		fit := int(city.ReadEvery / city.PulseGap)
		sc := open(t, city.Lively, with(state.Working, 0), with(state.Working, fit+15))

		t.Run("it should send as many as fit", func(t *testing.T) {
			assert.Len(t, starts(sc, read), fit)
		})

		t.Run("it should mark the last as standing for the rest", func(t *testing.T) {
			sc := open(t, city.Lively, with(state.Working, 0), with(state.Working, fit+15))
			assert.True(t, merged(sc, read))
		})
	})

	t.Run("when the count has not moved by the next read", func(t *testing.T) {
		sc := open(t, city.Lively, with(state.Working, 0), with(state.Working, 1), with(state.Working, 1))

		t.Run("it should send no new load", func(t *testing.T) {
			assert.Len(t, starts(sc, start.Add(2*city.ReadEvery)), 0)
		})
	})

	t.Run("when the map first opens on calls already made", func(t *testing.T) {
		sc := open(t, city.Lively, with(state.Working, 40))

		t.Run("it should send nothing", func(t *testing.T) {
			assert.Empty(t, starts(sc, start))
		})
	})

	t.Run("when the level is still", func(t *testing.T) {
		sc := open(t, city.Still, with(state.Working, 0), with(state.Working, 1))
		*sc.clock = read
		loads := sc.s.Loads("a")

		t.Run("a tool call should be a flash of the work lights, with no climb", func(t *testing.T) {
			require.Len(t, loads, 1)
			assert.True(t, loads[0].Flash)
		})

		t.Run("and the flash has run LoadFlashFor", func(t *testing.T) {
			*sc.clock = read.Add(city.LoadFlashFor)

			t.Run("it should be gone", func(t *testing.T) {
				assert.Empty(t, sc.s.Loads("a"))
			})
		})
	})

	t.Run("when the turn ends", func(t *testing.T) {
		sc := open(t, city.Lively, with(state.Working, 0), with(state.NeedsYou, 0))
		ended := start.Add(city.ReadEvery)

		t.Run("the scaffold should come down over ScaffoldDownFor", func(t *testing.T) {
			*sc.clock = ended.Add(city.ScaffoldDownFor / 2)
			assert.InDelta(t, 0.5, sc.s.Scaffold(building(sc)), 1e-9)
		})

		t.Run("while it comes down, the scene should be animating", func(t *testing.T) {
			*sc.clock = ended.Add(city.ScaffoldDownFor / 2)
			assert.True(t, sc.s.Animating())
		})

		t.Run("and ScaffoldDownFor has passed, it should be gone", func(t *testing.T) {
			*sc.clock = ended.Add(city.ScaffoldDownFor)
			assert.Zero(t, sc.s.Scaffold(building(sc)))
		})
	})

	t.Run("while a load climbs", func(t *testing.T) {
		sc := open(t, city.Lively, with(state.NeedsYou, 0), with(state.NeedsYou, 1))
		*sc.clock = read.Add(city.LoadFor / 2)

		t.Run("the scene should be animating", func(t *testing.T) {
			assert.True(t, sc.s.Animating())
		})
	})

	t.Run("when the building's card is read", func(t *testing.T) {
		edit := with(state.Working, 0)
		edit.Tool = "Edit"
		sc := open(t, city.Lively, edit)
		card, ok := sc.s.CardFor(city.Hit{Building: building(sc)})
		require.True(t, ok)

		t.Run("it should name the current tool", func(t *testing.T) {
			assert.Contains(t, strings.Join(card.Details, "\n"), "now      Edit")
		})
	})

	t.Run("the rover is gone", func(t *testing.T) {
		t.Run("no kind in the reference should be the rover", func(t *testing.T) {
			var rovers []string
			for _, k := range city.Kinds {
				if strings.Contains(k.Name(), "rover") || strings.Contains(k.About(), "rover") {
					rovers = append(rovers, k.Name())
				}
			}
			assert.Empty(t, rovers)
		})
	})
}
