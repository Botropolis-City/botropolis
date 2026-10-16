package render

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

func TestTriageKeys(t *testing.T) {
	now := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	waiting := func(id, cwd string, since time.Duration) state.Session {
		return state.Session{ID: id, CWD: cwd, State: state.NeedsYou, LastActivity: now.Add(-since)}
	}
	newGame := func(t *testing.T) (*Game, *recordingActor) {
		t.Helper()
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(state.Snapshot{At: now, Sessions: []state.Session{
			waiting("a", "/w/api", time.Hour), waiting("b", "/w/lib", 2*time.Hour), waiting("c", "/w/site", 3*time.Hour),
		}})
		actor := &recordingActor{}
		return &Game{scene: s, actor: actor, scripted: -1, clicked: -1}, actor
	}
	press := func(t *testing.T, g *Game, key ebiten.Key) {
		t.Helper()
		g.scripted = key
		require.NoError(t, g.handleKeys())
		g.scripted = -1
	}
	typed := func(t *testing.T, g *Game, s string) {
		t.Helper()
		for _, r := range s {
			g.scriptedRune = r
			require.NoError(t, g.handleKeys())
		}
		g.scriptedRune = 0
	}
	walking := func(t *testing.T) (*Game, *recordingActor) {
		t.Helper()
		g, actor := newGame(t)
		press(t, g, ebiten.KeyW)
		require.NotNil(t, g.triage(), "triage should be walking")
		return g, actor
	}
	current := func(g *Game) string {
		step, _ := g.scene.TriageStep()
		if step.Building == nil {
			return ""
		}
		return step.Building.Session.ID
	}

	t.Run("when w is pressed", func(t *testing.T) {
		g, _ := walking(t)

		t.Run("it should start on the longest-waiting", func(t *testing.T) {
			assert.Equal(t, "c", current(g))
		})
	})

	t.Run("while walking, n, p, d and s should do triage's thing and not their map meaning", func(t *testing.T) {
		cases := []struct {
			key      ebiten.Key
			name     string
			triage   func(g *Game, a *recordingActor) bool
			onTheMap func(g *Game, a *recordingActor, light city.Light) bool
		}{
			{ebiten.KeyN, "n moves on rather than cycling the light",
				func(g *Game, _ *recordingActor) bool { return current(g) == "b" },
				func(g *Game, _ *recordingActor, light city.Light) bool { return g.scene.Light() != light }},
			{ebiten.KeyP, "p rests the session rather than saving a screenshot",
				func(_ *Game, a *recordingActor) bool {
					return len(a.done) == 1 && reflect.DeepEqual(a.done[0], city.Action{Kind: city.ActionStop, SessionID: "c"})
				},
				func(g *Game, _ *recordingActor, _ city.Light) bool { return g.snap != "" }},
			{ebiten.KeyD, "d marks it done rather than demolishing it",
				func(g *Game, _ *recordingActor) bool { return len(g.scene.TriageWalk()) == 2 },
				func(g *Game, _ *recordingActor, _ city.Light) bool {
					return strings.Contains(g.notice.Text(timeNow()), "press d again")
				}},
			{ebiten.KeyS, "s opens the snooze rather than the settings",
				func(g *Game, _ *recordingActor) bool { return g.triage() != nil && g.triage().snoozing != nil },
				func(g *Game, _ *recordingActor, _ city.Light) bool { return g.settingsOpen }},
		}
		for _, tc := range cases {
			t.Run("when "+tc.name, func(t *testing.T) {
				g, actor := walking(t)
				light := g.scene.Light()
				press(t, g, tc.key)

				t.Run("it should do triage's thing", func(t *testing.T) {
					assert.True(t, tc.triage(g, actor))
				})

				t.Run("it should not do the map's", func(t *testing.T) {
					assert.False(t, tc.onTheMap(g, actor, light))
				})
			})
		}
	})

	t.Run("when a time is typed into the snooze", func(t *testing.T) {
		g, _ := walking(t)
		press(t, g, ebiten.KeyS)
		for range 3 {
			press(t, g, ebiten.KeyArrowDown)
		}
		typed(t, g, "in 2h")
		press(t, g, ebiten.KeyEnter)

		t.Run("it should snooze the session until then", func(t *testing.T) {
			assert.Len(t, g.scene.TriageWalk(), 2)
		})
	})

	t.Run("when a time that has passed is typed into the snooze", func(t *testing.T) {
		g, _ := walking(t)
		press(t, g, ebiten.KeyS)
		for range 3 {
			press(t, g, ebiten.KeyArrowDown)
		}
		typed(t, g, "08:00")
		press(t, g, ebiten.KeyEnter)

		t.Run("it should say why it was refused", func(t *testing.T) {
			require.NotNil(t, g.triage())
			assert.Contains(t, g.triage().snoozing.refused, "has passed")
		})
	})

	t.Run("when esc is pressed", func(t *testing.T) {
		g, _ := walking(t)
		press(t, g, ebiten.KeyEscape)

		t.Run("it should leave the walk", func(t *testing.T) {
			assert.Nil(t, g.triage())
		})
	})
}
