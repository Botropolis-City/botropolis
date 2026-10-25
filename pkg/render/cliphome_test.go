package render

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 76: a looping clip pans away and back to "home", where the camera
// stood when the clip began, so its last frame leads into its first.
func TestHomeTarget(t *testing.T) {
	newGame := func() (*Game, city.Point) {
		at := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
		var sessions []state.Session
		for i := range 6 {
			cwd := fmt.Sprintf("/p/d%d", i)
			sessions = append(sessions, state.Session{ID: cwd, CWD: cwd, State: state.Working, StartedAt: at})
		}
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return at }, time.UTC)
		s.SetInstant(true)
		s.Resize(1200, 800)
		s.SetSnapshot(state.Snapshot{At: at, Sessions: sessions})
		centre := city.Point{X: 600, Y: 400}
		return &Game{scene: s, scripted: -1, clicked: -1, player: newPlayer(nil, centre)}, centre
	}

	t.Run("when no clip has begun", func(t *testing.T) {
		g, _ := newGame()
		_, ok := g.resolveTarget("home")

		t.Run("it should find no home on the map", func(t *testing.T) {
			assert.False(t, ok)
		})
	})

	for _, away := range []city.Point{{X: 150, Y: -80}, {X: 50000, Y: 50000}} {
		t.Run(fmt.Sprintf("when a clip has begun and the camera is panned %v, as far as the city lets it", away), func(t *testing.T) {
			g, centre := newGame()
			began := g.scene.Camera().ScreenToWorld(centre)
			g.markHome()
			g.scene.Pan(away)
			home, ok := g.resolveTarget("home")
			if ok {
				g.scene.Pan(centre.Sub(home))
			}
			back := g.scene.Camera().ScreenToWorld(centre)

			t.Run("it should pan back to where the clip began", func(t *testing.T) {
				d := began.Sub(back)
				assert.InDelta(t, 0, math.Hypot(d.X, d.Y), 0.01)
			})
		})
	}
}
