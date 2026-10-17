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
	for name, keys := range map[string][]ui.Key{"help": bindings, "footer": footerKeys} {
		t.Run("the "+name+"'s f row should say Skyline", func(t *testing.T) {
			var action string
			for _, k := range keys {
				if k.Key == "f" {
					action = k.Action
				}
			}
			assert.Equal(t, "Skyline", action)
		})
	}

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
