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
// working floor while it is mid-turn, and one pulse up its face for each
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

		t.Run("it should have a working floor", func(t *testing.T) {
			_, ok := sc.s.WorkingFloor(building(sc))
			assert.True(t, ok)
		})
	})

	for _, st := range []state.State{state.NeedsYou, state.Waiting, state.Resting} {
		t.Run("when a session is "+string(st), func(t *testing.T) {
			sc := open(t, city.Lively, with(st, 0))

			t.Run("it should have no working floor", func(t *testing.T) {
				_, ok := sc.s.WorkingFloor(building(sc))
				assert.False(t, ok)
			})
		})
	}

	// starts is when each pulse a session's building ran began, watched
	// in steps from the last read until every one has finished.
	starts := func(sc scene, from time.Time) map[time.Duration]bool {
		seen := map[time.Duration]bool{}
		for at := from; at.Before(from.Add(city.ReadEvery + city.PulseFor)); at = at.Add(5 * time.Millisecond) {
			*sc.clock = at
			for _, p := range sc.s.Pulses("a") {
				began := at.Sub(from) - time.Duration(p.P*float64(city.PulseFor))
				seen[began.Round(10*time.Millisecond)] = true
			}
		}
		return seen
	}
	merged := func(sc scene, from time.Time) bool {
		for at := from; at.Before(from.Add(city.ReadEvery + city.PulseFor)); at = at.Add(5 * time.Millisecond) {
			*sc.clock = at
			for _, p := range sc.s.Pulses("a") {
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

		t.Run("it should send exactly one pulse", func(t *testing.T) {
			assert.Len(t, starts(sc, read), 1)
		})

		t.Run("and the pulse has run PulseFor", func(t *testing.T) {
			*sc.clock = read.Add(city.PulseFor)

			t.Run("it should be gone", func(t *testing.T) {
				assert.Empty(t, sc.s.Pulses("a"))
			})
		})
	})

	t.Run("when three tool calls are made between reads", func(t *testing.T) {
		sc := open(t, city.Lively, with(state.Working, 0), with(state.Working, 3))

		t.Run("it should send three pulses, spread across the interval", func(t *testing.T) {
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

		t.Run("it should send no new pulse", func(t *testing.T) {
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
		pulses := sc.s.Pulses("a")

		t.Run("a tool call should be a brief brightening, with no climb", func(t *testing.T) {
			require.Len(t, pulses, 1)
			assert.True(t, pulses[0].Brighten)
		})

		t.Run("and the brightening has run PulseFadeFor", func(t *testing.T) {
			*sc.clock = read.Add(city.PulseFadeFor)

			t.Run("it should be gone", func(t *testing.T) {
				assert.Empty(t, sc.s.Pulses("a"))
			})
		})
	})

	t.Run("while a pulse runs", func(t *testing.T) {
		sc := open(t, city.Lively, with(state.NeedsYou, 0), with(state.NeedsYou, 1))
		*sc.clock = read.Add(city.PulseFor / 2)

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
