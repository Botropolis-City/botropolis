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

// Item 352: feedback is at the point of action. A button whose action
// runs beside the loop says what it is doing in place of its label until
// it is done; a press with no such button on screen gets one notice.
func TestCardProgress(t *testing.T) {
	at := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	newGame := func(t *testing.T) (*Game, *blockingActor) {
		t.Helper()
		actor := &blockingActor{release: make(chan struct{}), reached: make(chan city.Action, 4)}
		scene := city.NewScene(city.NewLayout())
		scene.SetClock(func() time.Time { return at }, time.UTC)
		scene.Resize(1280, 800)
		scene.SetSnapshot(state.Snapshot{At: at, Sessions: []state.Session{
			{ID: "a", Title: "Fix the queue", CWD: "/src/kiln", State: state.Working, LastActivity: at},
			{ID: "b", Title: "Tide table", CWD: "/src/tide", State: state.Working, LastActivity: at},
		}})
		require.True(t, scene.Select("a"))
		g := &Game{scene: scene, actor: actor, scripted: -1, clicked: -1}
		t.Cleanup(func() {
			select {
			case <-actor.release:
			default:
				close(actor.release)
			}
			g.settle()
		})
		return g, actor
	}
	selected := func(g *Game) *city.Building { return g.scene.Selected() }
	labels := func(g *Game) []string {
		return g.progressLabels(g.pinnedVerbs, selected(g))
	}
	// shown puts the selected session's card on screen, attach direct and
	// stop and fork in its Session menu (item 350),
	// as the frame draws it.
	shown := func(g *Game) {
		g.pinned = ui.Card{Rect: city.RectAt(0, 0, 400, 200)}
		g.pinnedVerbs = city.Verbs{Direct: city.ActionAttach, Session: []city.ActionKind{city.ActionAttach, city.ActionStop, city.ActionFork}}
	}

	t.Run("when stop is running for the selected session", func(t *testing.T) {
		g, actor := newGame(t)
		shown(g)
		g.act(city.Action{Kind: city.ActionStop, SessionID: "a"})
		<-actor.reached

		t.Run("its stop button should say what it is doing, and fork keep its label", func(t *testing.T) {
			assert.Equal(t, []string{"attach", "stopping…", "Project ▾", "Group ▾"}, labels(g))
		})

		t.Run("it should show no notice, since the button says it", func(t *testing.T) {
			assert.Empty(t, g.notice.Text(timeNow()))
		})

		t.Run("and it ends", func(t *testing.T) {
			close(actor.release)
			g.settle()

			t.Run("the button should read Session again", func(t *testing.T) {
				assert.Equal(t, []string{"attach", "Session ▾", "Project ▾", "Group ▾"}, labels(g))
			})
		})
	})

	t.Run("when stop is running for another session", func(t *testing.T) {
		g, actor := newGame(t)
		shown(g)
		g.act(city.Action{Kind: city.ActionStop, SessionID: "b"})
		<-actor.reached

		t.Run("the selected card's buttons should keep their labels", func(t *testing.T) {
			assert.Equal(t, []string{"attach", "Session ▾", "Project ▾", "Group ▾"}, labels(g))
		})

		t.Run("it should say what it started, since no button on screen does", func(t *testing.T) {
			assert.Equal(t, "stopping Tide table…", g.notice.Text(timeNow()))
		})
	})

	t.Run("when New session starts a session from its panel", func(t *testing.T) {
		g, actor := newGame(t)
		shown(g)
		g.act(city.Action{Kind: city.ActionNewSession, Dir: "/src/kiln", Prompt: "go"})
		<-actor.reached

		t.Run("it should say what it started", func(t *testing.T) {
			assert.Equal(t, "starting a session in kiln…", g.notice.Text(timeNow()))
		})
	})

	t.Run("when the board's quiet sweep is running", func(t *testing.T) {
		g, _ := newGame(t)
		release := make(chan struct{})
		g.sweep = func([]string, time.Duration) (int, error) { <-release; return 0, nil }
		g.sweepIDs([]string{"a"}, time.Hour)

		t.Run("its Pack up button should say what it is doing", func(t *testing.T) {
			assert.Equal(t, "packing up…", g.quietButtonLabel(2))
		})

		t.Run("and it ends", func(t *testing.T) {
			close(release)
			g.settle()

			t.Run("it should read its label again", func(t *testing.T) {
				assert.Equal(t, quietLabel(2), g.quietButtonLabel(2))
			})
		})
	})
}
