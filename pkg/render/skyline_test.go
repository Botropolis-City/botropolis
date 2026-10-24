package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 89: f is Skyline, and at Skyline the buildings and their drones
// are drawn.
func TestSkylineKey(t *testing.T) {
	keyFor := func(keys []ui.Key, action string) string {
		for _, k := range keys {
			if k.Action == action {
				return k.Key
			}
		}
		return ""
	}

	t.Run("the help's Skyline row should list f and 0 as one key (item 181)", func(t *testing.T) {
		assert.Equal(t, "f or 0", keyFor(bindings, "Skyline, the whole city"))
	})

	t.Run("the help should list 0 on no other row (item 181)", func(t *testing.T) {
		var zeros []string
		for _, k := range bindings {
			if k.Key == "0" {
				zeros = append(zeros, k.Action)
			}
		}
		assert.Empty(t, zeros)
	})

	t.Run("the footer's f row should say Skyline", func(t *testing.T) {
		assert.Equal(t, "f", keyFor(footerKeys, "Skyline"))
	})

	t.Run("when a drone works on a building at Skyline", func(t *testing.T) {
		at := time.Date(2026, 10, 17, 18, 0, 0, 0, time.UTC)
		var sessions []state.Session
		for _, cwd := range []string{"/p/a", "/p/b", "/p/c", "/p/d", "/p/e", "/p/f", "/p/g", "/p/h"} {
			sessions = append(sessions, state.Session{ID: cwd, CWD: cwd, State: state.Working, StartedAt: at, SubagentsInFlight: 1})
		}
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return at }, time.UTC)
		s.SetProjection(city.Isometric)
		s.Resize(400, 280)
		s.SetSnapshot(state.Snapshot{At: at, Sessions: sessions})
		s.Skyline()
		g := &Game{scene: s}
		_, drones := g.flights(nil, s.City(), s.Camera(), 0)

		t.Run("it should be drawn", func(t *testing.T) {
			assert.Len(t, drones, len(sessions))
		})
	})
}
