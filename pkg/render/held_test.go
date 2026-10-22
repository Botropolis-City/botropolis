package render

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 197: a city nobody can see holds the daemon's snapshots instead
// of laying each out, and lays out the newest once it can be seen
// again; its title follows every snapshot, because a title is seen in a
// taskbar while the city is not. An unfocused city that can be seen is
// a dashboard (Aria, 2026-10-21): it lays out every snapshot and paints
// one frame for it, and nothing in between.
func TestAHiddenCityHoldsItsSnapshots(t *testing.T) {
	at := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	needing := func(n int) state.Snapshot {
		snap := state.Snapshot{At: at}
		for i := range 4 {
			st := state.Resting
			if i < n {
				st = state.NeedsYou
			}
			snap.Sessions = append(snap.Sessions, state.Session{ID: fmt.Sprintf("s%d", i), CWD: fmt.Sprintf("/home/user/src/p%d", i), State: st, StartedAt: at})
		}
		return snap
	}
	game := func(t *testing.T) *Game {
		t.Helper()
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return at }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(needing(0))
		return &Game{scene: s}
	}
	window := func(t *testing.T, focused, visible, minimised bool) {
		t.Helper()
		was := seen
		seen = func() (bool, bool, bool) { return focused, visible, minimised }
		t.Cleanup(func() { seen = was })
	}

	for _, unseen := range []struct {
		name                        string
		focused, visible, minimised bool
	}{
		{"when the window is unmapped, as i3's scratchpad leaves it", true, false, false},
		{"when the window is minimised", false, true, true},
	} {
		t.Run(unseen.name+", and three snapshots arrive", func(t *testing.T) {
			window(t, unseen.focused, unseen.visible, unseen.minimised)
			g := game(t)
			before := g.scene.Generation()
			for n := 1; n <= 3; n++ {
				snap := needing(n)
				g.takeSnapshot(&snap)
			}

			t.Run("it should not lay the city out", func(t *testing.T) {
				assert.Equal(t, before, g.scene.Generation())
			})

			t.Run("the window title should follow the newest snapshot", func(t *testing.T) {
				assert.Contains(t, g.shownTitle, "3 need you")
			})

			t.Run("it should paint no frame", func(t *testing.T) {
				assert.False(t, g.dashboardFrame())
			})

			t.Run("and the window is in front of someone again, and the next update comes", func(t *testing.T) {
				window(t, true, true, false)
				g.takeSnapshot(nil)

				t.Run("it should lay the city out once", func(t *testing.T) {
					assert.Equal(t, before+1, g.scene.Generation())
				})

				t.Run("it should lay out the newest snapshot", func(t *testing.T) {
					assert.Equal(t, needing(3), g.scene.Snapshot())
				})

				t.Run("and the update after it, it should not lay the city out again", func(t *testing.T) {
					g.takeSnapshot(nil)
					assert.Equal(t, before+1, g.scene.Generation())
				})
			})
		})
	}

	t.Run("when the window is visible but unfocused, as a dashboard on another monitor, and a snapshot arrives", func(t *testing.T) {
		window(t, false, true, false)
		g := game(t)
		before := g.scene.Generation()
		snap := needing(2)
		g.takeSnapshot(&snap)

		t.Run("it should lay it out", func(t *testing.T) {
			assert.Equal(t, before+1, g.scene.Generation())
		})

		t.Run("it should paint a frame for it", func(t *testing.T) {
			assert.True(t, g.dashboardFrame())
		})

		t.Run("and that frame is painted, it should paint no other until the next snapshot", func(t *testing.T) {
			assert.False(t, g.dashboardFrame())
		})
	})

	t.Run("when someone is watching, and a snapshot arrives", func(t *testing.T) {
		window(t, true, true, false)
		g := game(t)
		before := g.scene.Generation()
		snap := needing(2)
		g.takeSnapshot(&snap)

		t.Run("it should lay it out", func(t *testing.T) {
			assert.Equal(t, before+1, g.scene.Generation())
		})
	})
}
