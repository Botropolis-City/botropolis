package render

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/group"
	"botropolis.city/botropolis/pkg/state"
)

func TestNewSession(t *testing.T) {
	now := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	history := []group.Member{
		{ID: "a", Dir: "/w/lib", Started: now.Add(-9 * time.Hour), Ended: now.Add(-8 * time.Hour), Prompts: []string{"PROJ-1 start"}},
		{ID: "b", Dir: "/w/api", Started: now.Add(-6 * time.Hour), Ended: now.Add(-5 * time.Hour), Prompts: []string{"PROJ-1 bump"}},
		{ID: "c", Dir: "/w/logs", Started: now.Add(-4 * time.Hour), Ended: now.Add(-3 * time.Hour), Prompts: []string{"LOG-4 buffer"}},
		{ID: "d", Dir: "/w/config", Started: now.Add(-2 * time.Hour), Ended: now.Add(-90 * time.Minute), Prompts: []string{"LOG-4 schema"}},
	}
	guesser := group.NewGuesser(history, group.Memory{})
	idOf := func(session string) string {
		g, _ := group.Detect(history, group.Memory{}).GroupOf(session)
		return g.ID
	}
	newGame := func(t *testing.T) (*Game, *recordingActor) {
		t.Helper()
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(state.Snapshot{At: now, Sessions: []state.Session{{ID: "z", CWD: "/w/site", State: state.Working}}})
		actor := &recordingActor{}
		g := &Game{scene: s, actor: actor, scripted: -1, clicked: -1}
		g.groupSource = func() (*group.Guesser, []string) { return guesser, []string{"/w/api", "/w/lib"} }
		return g, actor
	}
	press := func(t *testing.T, g *Game, key ebiten.Key, shift bool) {
		t.Helper()
		g.scripted, g.scriptedShift = key, shift
		require.NoError(t, g.handleKeys())
		g.scripted, g.scriptedShift = -1, false
	}
	typed := func(t *testing.T, g *Game, s string) {
		t.Helper()
		for _, r := range s {
			g.scriptedRune = r
			require.NoError(t, g.handleKeys())
		}
		g.scriptedRune = 0
	}
	opened := func(t *testing.T) (*Game, *recordingActor) {
		t.Helper()
		g, actor := newGame(t)
		press(t, g, ebiten.KeyN, true)
		require.NotNil(t, g.newSession(), "New session should be open")
		return g, actor
	}

	t.Run("when n is pressed", func(t *testing.T) {
		g, _ := newGame(t)
		before := g.scene.Light()
		press(t, g, ebiten.KeyN, false)

		t.Run("it should cycle the light", func(t *testing.T) {
			assert.NotEqual(t, before, g.scene.Light())
		})
	})

	t.Run("when Shift+N is pressed", func(t *testing.T) {
		t.Run("it should open New session", func(t *testing.T) {
			g, _ := newGame(t)
			press(t, g, ebiten.KeyN, true)
			assert.NotNil(t, g.newSession())
		})

		t.Run("the light should not change", func(t *testing.T) {
			g, _ := newGame(t)
			before := g.scene.Light()
			press(t, g, ebiten.KeyN, true)
			assert.Equal(t, before, g.scene.Light())
		})
	})

	t.Run("when a --keys script presses shift+n", func(t *testing.T) {
		g, _ := newGame(t)
		g.pressScripted("shift+n")
		require.NoError(t, g.handleKeys())

		t.Run("it should open New session", func(t *testing.T) {
			assert.NotNil(t, g.newSession())
		})
	})

	t.Run("when a --keys script types a character into New session", func(t *testing.T) {
		g, _ := opened(t)
		g.pressScripted("x")
		require.NoError(t, g.handleKeys())

		t.Run("it should be typed into the field", func(t *testing.T) {
			assert.Equal(t, "x", g.newSession().fields[fieldDir].query())
		})
	})

	t.Run("when the board's New session button is clicked", func(t *testing.T) {
		g, _ := newGame(t)
		g.openBoard()
		b := g.board()
		b.newButton = city.RectAt(10, 10, 100, 20)
		g.clickOverlay(city.Point{X: 20, Y: 15})

		t.Run("it should open New session", func(t *testing.T) {
			assert.NotNil(t, g.newSession())
		})
	})

	t.Run("when a building is selected and New session opens", func(t *testing.T) {
		g, _ := newGame(t)
		b := g.scene.City().Buildings()[0]
		g.scene.Click(g.scene.Camera().WorldToScreen(b.Rect.Center()))
		require.NotNil(t, g.scene.Selected())
		press(t, g, ebiten.KeyN, true)

		t.Run("it should prefill its district's directory, editable", func(t *testing.T) {
			assert.Equal(t, "/w/site", g.newSession().fields[fieldDir].query())
		})
	})

	t.Run("when New session is open", func(t *testing.T) {
		t.Run("and a prompt naming a group's reference is typed", func(t *testing.T) {
			g, _ := opened(t)
			typed(t, g, "/w/infra")
			press(t, g, ebiten.KeyEnter, false)
			typed(t, g, "carry on with LOG-4")

			t.Run("it should guess that group", func(t *testing.T) {
				assert.Equal(t, idOf("c"), g.newSession().choice().Group)
			})
		})

		t.Run("and Tab is pressed in the group field", func(t *testing.T) {
			g, _ := opened(t)
			typed(t, g, "/w/infra")
			press(t, g, ebiten.KeyEnter, false)
			typed(t, g, "carry on with LOG-4")
			press(t, g, ebiten.KeyArrowDown, false)
			press(t, g, ebiten.KeyTab, false)

			t.Run("it should walk to the next alternative", func(t *testing.T) {
				assert.Equal(t, idOf("a"), g.newSession().choice().Group)
			})
		})

		t.Run("and a name is typed in the group field", func(t *testing.T) {
			g, _ := opened(t)
			typed(t, g, "/w/infra")
			press(t, g, ebiten.KeyEnter, false)
			typed(t, g, "go")
			press(t, g, ebiten.KeyArrowDown, false)
			typed(t, g, "Gateway")

			t.Run("it should make a new group with that name", func(t *testing.T) {
				assert.Equal(t, city.Action{GroupName: "Gateway"}, g.newSession().choice())
			})
		})

		t.Run("and enter starts it", func(t *testing.T) {
			g, actor := opened(t)
			typed(t, g, "/w/infra")
			press(t, g, ebiten.KeyEnter, false)
			typed(t, g, "carry on with LOG-4")
			press(t, g, ebiten.KeyEnter, false)

			t.Run("it should hand the actor the directory, prompt and group", func(t *testing.T) {
				assert.Equal(t, []city.Action{{Kind: city.ActionNewSession, Dir: "/w/infra", Prompt: "carry on with LOG-4", Group: idOf("c")}}, actor.done)
			})
		})

		t.Run("and esc is pressed", func(t *testing.T) {
			g, actor := opened(t)
			press(t, g, ebiten.KeyEscape, false)

			t.Run("it should close", func(t *testing.T) {
				assert.Nil(t, g.newSession())
			})

			t.Run("it should start nothing", func(t *testing.T) {
				assert.Empty(t, actor.done)
			})
		})
	})
}

func TestCompleteDir(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"alpha", "beta", "gamma"} {
		require.NoError(t, os.Mkdir(filepath.Join(root, d), 0o755))
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, "afile"), nil, 0o600))
	recent := []string{filepath.Join(root, "gamma"), "/elsewhere/beta"}

	t.Run("when Tab is pressed on a prefix", func(t *testing.T) {
		t.Run("it should offer recently used directories first, then the filesystem's, and no files", func(t *testing.T) {
			assert.Equal(t, []string{filepath.Join(root, "gamma"), filepath.Join(root, "alpha"), filepath.Join(root, "beta")},
				completeDir(root+"/", recent))
		})
	})

	t.Run("when the prefix narrows the names", func(t *testing.T) {
		t.Run("it should offer only those that match", func(t *testing.T) {
			assert.Equal(t, []string{filepath.Join(root, "alpha")}, completeDir(filepath.Join(root, "al"), recent))
		})
	})
}
