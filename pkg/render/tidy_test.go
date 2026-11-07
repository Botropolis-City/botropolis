package render

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 333: a group's members named and counted as the map shows them,
// a group renamed from the board or a card, and a session taken out of
// its group from the dive.
func TestCorrectGroups(t *testing.T) {
	now := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
	member := func(id, cwd, gid string, started time.Duration) state.Session {
		return state.Session{ID: id, Title: "Title " + id, CWD: cwd, State: state.Working, Group: gid,
			StartedAt: now.Add(started), LastActivity: now.Add(started / 2)}
	}
	groups := []state.Group{
		{ID: "g1", Name: "PROJ-9 · limiter", Members: []string{"a", "b", "c"}, MemberDirs: []string{"/w/lib", "/w/api", "/w/infra"},
			MemberTitles: []string{"Title a", "Title b", "Title c"}, Move: state.MoveAgents, Why: "a is working",
			Journal: state.Journal{LastTime: "x", Next: "y", Active: now.Add(-time.Hour)}},
		{ID: "g2", Name: "LOG-88 · buffer", Members: []string{"d", "e"}, MemberDirs: []string{"/w/site", "/w/docs"},
			MemberTitles: []string{"Title d", "Title e"}, Move: state.MoveAgents, Why: "d is working",
			Journal: state.Journal{LastTime: "x", Next: "y", Active: now.Add(-2 * time.Hour)}},
	}
	newGame := func(t *testing.T) (*Game, *recordingActor) {
		t.Helper()
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(1280, 800)
		scripted := member("c", "/w/infra", "g1", -time.Hour)
		scripted.Entrypoint = "sdk-cli"
		s.SetSnapshot(state.Snapshot{At: now, Groups: groups, Sessions: []state.Session{
			member("a", "/w/lib", "g1", -3*time.Hour), member("b", "/w/api", "g1", -2*time.Hour), scripted,
			member("d", "/w/site", "g2", -time.Hour), member("e", "/w/docs", "g2", -time.Hour),
		}})
		actor := &recordingActor{}
		return &Game{scene: s, actor: actor, theme: ui.NewTheme(1), scripted: -1, clicked: -1,
			dispatch:         func(run func()) { run() },
			readConversation: func(string, int, int) (claude.Conversation, error) { return claude.Conversation{}, nil },
		}, actor
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
			g.scriptedRune = 0
		}
	}
	cleared := func(t *testing.T, g *Game) {
		t.Helper()
		for range len([]rune(g.picker().name.query())) {
			press(t, g, ebiten.KeyBackspace)
		}
	}
	renaming := func(t *testing.T) (*Game, *recordingActor) {
		t.Helper()
		g, actor := newGame(t)
		press(t, g, ebiten.KeyG)
		require.NotNil(t, g.board())
		press(t, g, ebiten.KeyF2)
		require.NotNil(t, g.picker(), "the rename panel should be open")
		return g, actor
	}

	t.Run("when the board lists a group with a member off the map", func(t *testing.T) {
		g, _ := newGame(t)
		row := boardRows(g.scene.Groups(), now)[0]

		t.Run("it should name only the members on the map", func(t *testing.T) {
			assert.Equal(t, "Title a, Title b", row.Members)
		})

		t.Run("it should count only those", func(t *testing.T) {
			assert.Equal(t, "2 sessions", row.Count)
		})
	})

	t.Run("when F2 is pressed on the board", func(t *testing.T) {
		g, _ := renaming(t)

		t.Run("it should offer the chosen group's name to edit", func(t *testing.T) {
			assert.Equal(t, "PROJ-9 · limiter", g.picker().name.query())
		})

		t.Run("it should say it renames", func(t *testing.T) {
			assert.Contains(t, g.picker().title(), "rename group")
		})

		t.Run("and a new name is typed and enter pressed", func(t *testing.T) {
			g, actor := renaming(t)
			cleared(t, g)
			typed(t, g, " The limiter ")
			press(t, g, ebiten.KeyEnter)

			t.Run("it should rename the group to the trimmed name", func(t *testing.T) {
				assert.Equal(t, []city.Action{{Kind: city.ActionRename, Group: "g1", GroupName: "The limiter"}}, actor.done)
			})

			t.Run("it should close the panel", func(t *testing.T) {
				assert.Nil(t, g.picker())
			})
		})

		for _, c := range []struct{ when, name, refused string }{
			{"the name is emptied", "", "a group needs a name"},
			{"another group's name is typed", "log-88 · BUFFER", "LOG-88 · buffer is already a group"},
		} {
			t.Run("and "+c.when, func(t *testing.T) {
				g, actor := renaming(t)
				cleared(t, g)
				typed(t, g, c.name)
				press(t, g, ebiten.KeyEnter)

				t.Run("it should refuse, saying why", func(t *testing.T) {
					assert.Equal(t, c.refused, g.picker().refused)
				})

				t.Run("it should rename nothing", func(t *testing.T) {
					assert.Empty(t, actor.done)
				})
			})
		}

		t.Run("and only its case is changed", func(t *testing.T) {
			g, actor := renaming(t)
			cleared(t, g)
			typed(t, g, "proj-9 · LIMITER")
			press(t, g, ebiten.KeyEnter)

			t.Run("it should rename it, since the name it has is its own", func(t *testing.T) {
				assert.Equal(t, []city.Action{{Kind: city.ActionRename, Group: "g1", GroupName: "proj-9 · LIMITER"}}, actor.done)
			})
		})

		t.Run("and esc is pressed", func(t *testing.T) {
			g, actor := renaming(t)
			press(t, g, ebiten.KeyEscape)

			t.Run("it should close the panel and rename nothing", func(t *testing.T) {
				assert.Equal(t, [2]any{true, 0}, [2]any{g.picker() == nil, len(actor.done)})
			})
		})
	})

	t.Run("when rename group is chosen on a card", func(t *testing.T) {
		g, _ := newGame(t)
		g.act(city.Action{Kind: city.ActionRenameGroup, Group: "g2", GroupName: "LOG-88 · buffer"})

		t.Run("it should open the rename panel on that group's name", func(t *testing.T) {
			require.NotNil(t, g.picker())
			assert.Equal(t, [2]string{"g2", "LOG-88 · buffer"}, [2]string{g.picker().renaming, g.picker().name.query()})
		})
	})

	t.Run("when a group is focused and a row of its list is stepped", func(t *testing.T) {
		diving := func(t *testing.T) (*Game, *recordingActor) {
			t.Helper()
			g, actor := newGame(t)
			require.True(t, g.scene.FocusGroup("g1"))
			press(t, g, ebiten.KeyJ)
			require.GreaterOrEqual(t, g.scene.DiveStep(), 0)
			return g, actor
		}
		g, _ := diving(t)
		lead := g.scene.Dive()[g.scene.DiveStep()].Session

		t.Run("the hold should cap the stepped row, whose session - takes out, with - (item 351)", func(t *testing.T) {
			row := city.RectAt(0, 0, 200, 40)
			assert.Equal(t, []ui.Control{{Name: "take out", Rect: row, Keys: []string{"-"}}}, diveControls(row, lead))
		})

		t.Run("and no session would be taken out, the hold should cap nothing there", func(t *testing.T) {
			assert.Empty(t, diveControls(city.RectAt(0, 0, 200, 40), ""))
		})

		t.Run("and - is pressed", func(t *testing.T) {
			g, actor := diving(t)
			press(t, g, ebiten.KeyMinus)

			t.Run("it should take the row's session out of the group", func(t *testing.T) {
				assert.Equal(t, []city.Action{{Kind: city.ActionTakeOut, SessionID: lead, Group: "g1"}}, actor.done)
			})
		})
	})

	t.Run("when a group is focused and no row is stepped", func(t *testing.T) {
		g, actor := newGame(t)
		require.True(t, g.scene.FocusGroup("g1"))
		g.scripted = ebiten.KeyMinus
		took := g.diveKeys()
		g.scripted = -1

		t.Run("the dive should leave - to the map, to zoom", func(t *testing.T) {
			assert.False(t, took)
		})

		t.Run("it should take nobody out", func(t *testing.T) {
			assert.Empty(t, actor.done)
		})
	})

	t.Run("when Delete is pressed on the board", func(t *testing.T) {
		armed := func(t *testing.T) (*Game, *recordingActor) {
			t.Helper()
			g, actor := newGame(t)
			g.scene.SetClock(func() time.Time { return now }, time.UTC)
			press(t, g, ebiten.KeyG)
			require.NotNil(t, g.board())
			press(t, g, ebiten.KeyDelete)
			return g, actor
		}
		g, actor := armed(t)

		t.Run("it should close nothing yet", func(t *testing.T) {
			assert.Empty(t, actor.done)
		})

		// Item 352: the question is a dialog, and Delete again answers it.
		t.Run("it should open the dialog naming the group, saying what closing does", func(t *testing.T) {
			require.NotNil(t, g.dialog())
			assert.Equal(t, []string{"Close PROJ-9 · limiter?", city.ActionCloseGroup.Describe()}, append([]string{g.dialog().title}, g.dialog().lines...))
		})

		t.Run("and Delete is pressed again", func(t *testing.T) {
			g, actor := armed(t)
			press(t, g, ebiten.KeyDelete)

			t.Run("it should close the group, packing every member", func(t *testing.T) {
				assert.Equal(t, []city.Action{{Kind: city.ActionCloseGroup, Group: "g1", Sessions: []string{"a", "b", "c"}}}, actor.done)
			})

			t.Run("it should close the dialog and keep the board", func(t *testing.T) {
				assert.True(t, g.dialog() == nil && g.board() != nil)
			})
		})

		t.Run("and esc is pressed", func(t *testing.T) {
			g, actor := armed(t)
			press(t, g, ebiten.KeyEscape)

			t.Run("it should close the dialog and nothing else", func(t *testing.T) {
				assert.True(t, g.dialog() == nil && g.board() != nil && len(actor.done) == 0, "dialog %v, board %v, done %v", g.dialog(), g.board(), actor.done)
			})
		})
	})

	t.Run("when a group's every pull request merged", func(t *testing.T) {
		g, _ := newGame(t)
		snap := g.scene.RealSnapshot()
		snap.Sessions = append([]state.Session(nil), snap.Sessions...)
		for i := range snap.Sessions {
			if snap.Sessions[i].Group == "g2" {
				snap.Sessions[i].PRs = []claude.PR{{Number: i, Repository: "o/r", State: claude.PRMerged}}
			}
		}
		g.scene.SetSnapshot(snap)
		rows := boardRows(g.scene.Groups(), now)

		t.Run("its row should offer to close it", func(t *testing.T) {
			assert.Equal(t, "every pull request merged: Delete closes it", g.closeOffer(g.scene.Groups()[1].ID))
		})

		t.Run("a row with work still open should offer nothing", func(t *testing.T) {
			assert.Empty(t, g.closeOffer(g.scene.Groups()[0].ID), "%v", rows)
		})
	})

	t.Run("when the help lists the keys", func(t *testing.T) {
		actions := map[string]string{}
		for _, k := range bindings {
			actions[k.Key] = k.Action
		}

		t.Run("it should say F2 renames a group on the board", func(t *testing.T) {
			assert.Contains(t, actions["F2"], "rename")
		})
	})
}
