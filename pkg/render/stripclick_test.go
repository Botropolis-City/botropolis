package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 352: the needs-you count on the strip is the button the footer's
// needs-you line was: a click attaches the session that has waited
// longest. The other counts still select the next session in theirs.
func TestStripClick(t *testing.T) {
	at := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	newGame := func(t *testing.T) (*Game, *recordingActor) {
		t.Helper()
		actor := &recordingActor{}
		scene := city.NewScene(city.NewLayout())
		scene.SetClock(func() time.Time { return at }, time.UTC)
		scene.Resize(1280, 800)
		scene.SetSnapshot(state.Snapshot{At: at, Sessions: []state.Session{
			{ID: "newer", Title: "Newer", CWD: "/src/a", State: state.NeedsYou, LastActivity: at.Add(-time.Minute)},
			{ID: "older", Title: "Older", CWD: "/src/b", State: state.NeedsYou, LastActivity: at.Add(-time.Hour)},
			{ID: "busy", Title: "Busy", CWD: "/src/c", State: state.Working, LastActivity: at},
		}})
		g := &Game{scene: scene, actor: actor, scripted: -1, clicked: -1}
		g.stripLayout = ui.LayoutStrip(ui.NewTheme(1), ui.StripChips(scene.Summary()), 1280, func(s string, _ ui.Size) (float64, float64) {
			return 7 * float64(len([]rune(s))), 16
		})
		t.Cleanup(g.settle)
		return g, actor
	}
	chip := func(t *testing.T, g *Game, st state.State) city.Point {
		t.Helper()
		for _, c := range g.stripLayout.Chips {
			if c.State == st {
				return c.Rect.Center()
			}
		}
		require.Failf(t, "no chip", "%s", st)
		return city.Point{}
	}

	t.Run("when the needs-you chip is clicked", func(t *testing.T) {
		g, actor := newGame(t)
		require.True(t, g.stripClick(chip(t, g, state.NeedsYou)))
		g.settle()

		t.Run("it should attach the session that has waited longest", func(t *testing.T) {
			require.NotEmpty(t, actor.done)
			assert.Equal(t, [2]string{string(city.ActionAttach), "older"}, [2]string{string(actor.done[0].Kind), actor.done[0].SessionID})
		})
	})

	t.Run("when another state's chip is clicked", func(t *testing.T) {
		g, actor := newGame(t)
		require.True(t, g.stripClick(chip(t, g, state.Working)))

		t.Run("it should select the next session in that state", func(t *testing.T) {
			require.NotNil(t, g.scene.Selected())
			assert.Equal(t, "busy", g.scene.Selected().Session.ID)
		})

		t.Run("it should attach nothing", func(t *testing.T) {
			assert.Empty(t, actor.done)
		})
	})
}
