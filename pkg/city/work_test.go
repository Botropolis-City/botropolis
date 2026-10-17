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
	with := func(st state.State, tool string) state.Session {
		s := session("a", cinders, st)
		s.SubagentsInFlight, s.Tool = 0, tool
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
		for _, step := range steps {
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
		sc := open(t, city.Lively, with(state.Working, ""))

		t.Run("it should have a working floor", func(t *testing.T) {
			_, ok := sc.s.WorkingFloor(building(sc))
			assert.True(t, ok)
		})
	})

	for _, st := range []state.State{state.NeedsYou, state.Waiting, state.Resting} {
		t.Run("when a session is "+string(st), func(t *testing.T) {
			sc := open(t, city.Lively, with(st, ""))

			t.Run("it should have no working floor", func(t *testing.T) {
				_, ok := sc.s.WorkingFloor(building(sc))
				assert.False(t, ok)
			})
		})
	}

	t.Run("when a tool call starts", func(t *testing.T) {
		sc := open(t, city.Lively, with(state.Working, ""), with(state.Working, "Edit"))

		t.Run("it should send exactly one pulse", func(t *testing.T) {
			assert.Len(t, sc.s.Pulses("a"), 1)
		})

		t.Run("and the pulse has run PulseFor", func(t *testing.T) {
			*sc.clock = start.Add(city.PulseFor)

			t.Run("it should be gone", func(t *testing.T) {
				assert.Empty(t, sc.s.Pulses("a"))
			})
		})
	})

	t.Run("when the session moves straight on to another tool", func(t *testing.T) {
		sc := open(t, city.Lively, with(state.Working, ""), with(state.Working, "Edit"), with(state.Working, "Bash"))

		t.Run("it should send another pulse", func(t *testing.T) {
			assert.Len(t, sc.s.Pulses("a"), 2)
		})
	})

	t.Run("when the same call is still running at the next snapshot", func(t *testing.T) {
		sc := open(t, city.Lively, with(state.Working, ""), with(state.Working, "Edit"), with(state.Working, "Edit"))

		t.Run("it should send no new pulse", func(t *testing.T) {
			assert.Len(t, sc.s.Pulses("a"), 1)
		})
	})

	t.Run("when the map first opens on a call already running", func(t *testing.T) {
		sc := open(t, city.Lively, with(state.Working, "Edit"))

		t.Run("it should send nothing", func(t *testing.T) {
			assert.Empty(t, sc.s.Pulses("a"))
		})
	})

	t.Run("when the level is still", func(t *testing.T) {
		sc := open(t, city.Still, with(state.Working, ""), with(state.Working, "Edit"))
		pulses := sc.s.Pulses("a")

		t.Run("a tool call should be a brief brightening, with no climb", func(t *testing.T) {
			require.Len(t, pulses, 1)
			assert.True(t, pulses[0].Brighten)
		})

		t.Run("and the brightening has run PulseFadeFor", func(t *testing.T) {
			*sc.clock = start.Add(city.PulseFadeFor)

			t.Run("it should be gone", func(t *testing.T) {
				assert.Empty(t, sc.s.Pulses("a"))
			})
		})
	})

	t.Run("while a pulse runs", func(t *testing.T) {
		sc := open(t, city.Lively, with(state.NeedsYou, ""), with(state.NeedsYou, "Edit"))
		*sc.clock = start.Add(city.PulseFor / 2)

		t.Run("the scene should be animating", func(t *testing.T) {
			assert.True(t, sc.s.Animating())
		})
	})

	t.Run("when the building's card is read", func(t *testing.T) {
		sc := open(t, city.Lively, with(state.Working, "Edit"))
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
