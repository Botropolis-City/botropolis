package render

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

func TestBoard(t *testing.T) {
	now := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	member := func(id, cwd, group string) state.Session {
		return state.Session{ID: id, CWD: cwd, State: state.Working, Group: group, LastActivity: now}
	}
	groups := []state.Group{
		{ID: "g1", Name: "PROJ-123 · rate-limit headers", Short: "PROJ-123", Members: []string{"a", "b"},
			Move: state.MoveYours, Why: "a needs you",
			Journal: state.Journal{LastTime: "finished the client", Next: "tag v1.4.0", Active: now.Add(-3 * time.Hour)}},
		{ID: "g2", Name: "LOG-88 · buffer", Short: "LOG-88", Members: []string{"c"},
			Move: state.MoveAgents, Why: "c is working",
			Journal: state.Journal{LastTime: "capped the buffer", Next: "not recorded", Active: now.Add(-time.Hour)}},
	}
	newGame := func(groups []state.Group) *Game {
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(state.Snapshot{At: now, Groups: groups, Sessions: []state.Session{
			member("a", "/w/lib", "g1"), member("b", "/w/api", "g1"), member("c", "/w/config", "g2"),
			member("d", "/w/site", ""), member("e", "/w/docs", ""),
		}})
		return &Game{scene: s, scripted: -1, clicked: -1}
	}
	press := func(t *testing.T, g *Game, key ebiten.Key) {
		t.Helper()
		g.scripted = key
		require.NoError(t, g.handleKeys())
		g.scripted = -1
	}
	opened := func(t *testing.T) *Game {
		t.Helper()
		g := newGame(groups)
		press(t, g, ebiten.KeyG)
		require.NotNil(t, g.board(), "the board should be open")
		return g
	}

	t.Run("when g is pressed", func(t *testing.T) {
		g := newGame(groups)
		press(t, g, ebiten.KeyG)

		t.Run("it should open the board", func(t *testing.T) {
			assert.NotNil(t, g.board())
		})
	})

	t.Run("when City Hall is clicked", func(t *testing.T) {
		g := newGame(groups)
		g.clickLandmark(city.LandmarkHall)

		t.Run("it should open the board", func(t *testing.T) {
			assert.NotNil(t, g.board())
		})
	})

	t.Run("when the board is open", func(t *testing.T) {
		t.Run("it should have one row per group, saying whose move it is, how long it has waited, last time and next", func(t *testing.T) {
			assert.Equal(t, []boardRow{
				{Name: "PROJ-123 · rate-limit headers", Move: "yours: a needs you", Waited: "3h00m", LastTime: "finished the client", Next: "tag v1.4.0"},
				{Name: "LOG-88 · buffer", Move: "the agents': c is working", Waited: "1h00m", LastTime: "capped the buffer", Next: "not recorded"},
			}, boardRows(groups, now))
		})

		t.Run("and down is pressed", func(t *testing.T) {
			g := opened(t)
			press(t, g, ebiten.KeyArrowDown)

			t.Run("it should select the next row", func(t *testing.T) {
				assert.Equal(t, 1, g.board().selected)
			})

			t.Run("and up is pressed", func(t *testing.T) {
				press(t, g, ebiten.KeyArrowUp)

				t.Run("it should select the row before", func(t *testing.T) {
					assert.Equal(t, 0, g.board().selected)
				})
			})
		})

		t.Run("and enter is pressed", func(t *testing.T) {
			g := opened(t)
			press(t, g, ebiten.KeyArrowDown)
			press(t, g, ebiten.KeyEnter)

			t.Run("it should focus the selected group", func(t *testing.T) {
				assert.Equal(t, "g2", g.scene.FocusedGroup())
			})

			t.Run("it should close the board", func(t *testing.T) {
				assert.Nil(t, g.board())
			})

			t.Run("and esc is pressed", func(t *testing.T) {
				press(t, g, ebiten.KeyEscape)

				t.Run("it should clear the focus", func(t *testing.T) {
					assert.Equal(t, "", g.scene.FocusedGroup())
				})
			})
		})

		t.Run("and esc is pressed", func(t *testing.T) {
			g := opened(t)
			press(t, g, ebiten.KeyEscape)

			t.Run("it should close the board", func(t *testing.T) {
				assert.Nil(t, g.board())
			})

			t.Run("it should focus nothing", func(t *testing.T) {
				assert.Equal(t, "", g.scene.FocusedGroup())
			})
		})
	})

	t.Run("when there are no groups", func(t *testing.T) {
		t.Run("it should say so", func(t *testing.T) {
			assert.Equal(t, "no groups yet: sessions join one when they share a ticket, a link or their wording", boardStatus(nil))
		})
	})

	t.Run("when there are groups", func(t *testing.T) {
		t.Run("it should say how to use the board", func(t *testing.T) {
			assert.Equal(t, "2 groups   ↑ ↓ choose, enter focuses it on the map, c copies its stand-up, esc closes", boardStatus(boardRows(groups, now)))
		})
	})
}

// Item 90: c on the board copies the chosen group's stand-up line.
func TestBoardStandup(t *testing.T) {
	now := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	actor := &recorder{}
	s := city.NewScene(city.NewLayout())
	s.SetClock(func() time.Time { return now }, time.UTC)
	s.Resize(800, 600)
	s.SetSnapshot(state.Snapshot{At: now,
		Groups: []state.Group{{ID: "g1", Name: "PROJ-123", Short: "PROJ-123", Members: []string{"a"}}},
		Sessions: []state.Session{{ID: "a", CWD: "/w/lib", State: state.Working, Group: "g1", StartedAt: now, LastActivity: now},
			{ID: "d", CWD: "/w/site", State: state.Working}}})
	g := &Game{scene: s, actor: actor, scripted: -1, clicked: -1}
	pressKey(t, g, ebiten.KeyG, false)
	require.NotNil(t, g.board(), "the board should be open")

	t.Run("when the board is open and c is pressed", func(t *testing.T) {
		pressKey(t, g, ebiten.KeyC, false)
		g.settle()

		t.Run("it should copy the selected row's stand-up", func(t *testing.T) {
			assert.Equal(t, []city.Action{{Kind: city.ActionCopy, Text: "lib in progress"}}, actor.done)
		})
	})
}
