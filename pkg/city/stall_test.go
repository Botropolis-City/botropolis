package city_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 273, slice 2: a scripted run shown on the map is a stall, and its
// subagents in flight queue at it as customers instead of flying over it
// as drones.
func TestStall(t *testing.T) {
	shown := func(t *testing.T, sessions ...state.Session) *city.Scene {
		t.Helper()
		s := scene(t)
		s.SetShown(city.Shown{Scripted: true})
		s.SetSnapshot(snapshot(sessions...))
		return s
	}
	find := func(t *testing.T, s *city.Scene, id string) *city.Building {
		t.Helper()
		for _, b := range s.City().Buildings() {
			if b.Session.ID == id {
				return b
			}
		}
		require.FailNow(t, "no building for "+id)
		return nil
	}
	busy := func(id, entrypoint string, inFlight int) state.Session {
		s := session(id, "/src/pipeline", state.Working)
		s.Entrypoint = entrypoint
		s.SubagentsInFlight = inFlight
		return s
	}

	for _, c := range []struct {
		entrypoint string
		want       bool
	}{{"sdk-cli", true}, {"cli", false}, {"", false}} {
		t.Run(fmt.Sprintf("when a session's entrypoint is %q", c.entrypoint), func(t *testing.T) {
			s := shown(t, busy("a", c.entrypoint, 0))

			t.Run(fmt.Sprintf("it should be a stall is %v", c.want), func(t *testing.T) {
				assert.Equal(t, c.want, find(t, s, "a").Stall())
			})
		})
	}

	t.Run("when a scripted session has subagents in flight", func(t *testing.T) {
		s := shown(t, busy("a", "sdk-cli", 3))
		b := find(t, s, "a")

		t.Run("it should queue one customer per subagent", func(t *testing.T) {
			assert.Len(t, s.Queue(b), 3)
		})

		t.Run("it should fly no drone over it", func(t *testing.T) {
			assert.Empty(t, s.Flights("a"))
		})

		t.Run("it should stand every customer on its lot, clear of the stall", func(t *testing.T) {
			var astray int
			for _, p := range s.Queue(b) {
				if !b.Rect.Contains(p) || b.StallBody().Contains(p) {
					astray++
				}
			}
			assert.Zero(t, astray)
		})
	})

	t.Run("when a scripted session has more subagents in flight than a queue holds", func(t *testing.T) {
		s := shown(t, busy("a", "sdk-cli", city.MaxQueue+4))

		t.Run("it should queue as many as it holds", func(t *testing.T) {
			assert.Len(t, s.Queue(find(t, s, "a")), city.MaxQueue)
		})
	})

	t.Run("when an interactive session has subagents in flight", func(t *testing.T) {
		s := shown(t, busy("a", "cli", 2))

		t.Run("it should queue no customer", func(t *testing.T) {
			assert.Empty(t, s.Queue(find(t, s, "a")))
		})
	})
}
