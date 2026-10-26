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

type recordingActor struct{ done []city.Action }

func (a *recordingActor) Do(action city.Action) error {
	a.done = append(a.done, action)
	return nil
}

func TestMoveToGroup(t *testing.T) {
	now := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	groups := []state.Group{{ID: "g1", Name: "PROJ-123 · headers"}, {ID: "g2", Name: "LOG-88 · buffer"}}
	newGame := func() (*Game, *recordingActor) {
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(state.Snapshot{At: now, Groups: groups, Sessions: []state.Session{
			{ID: "a", CWD: "/w/lib", State: state.Working, Group: "g1"},
			{ID: "b", CWD: "/w/api", State: state.Working, Group: "g2"},
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
	picking := func(t *testing.T) (*Game, *recordingActor) {
		t.Helper()
		g, actor := newGame()
		g.act(city.Action{Kind: city.ActionMoveToGroup, SessionID: "a"})
		require.NotNil(t, g.picker(), "the picker should be open")
		return g, actor
	}

	t.Run("when move to group is chosen on a card", func(t *testing.T) {
		g, _ := picking(t)

		t.Run("it should offer new group…, then every group, then no group", func(t *testing.T) {
			assert.Equal(t, []string{"new group…", "PROJ-123 · headers", "LOG-88 · buffer", "no group"}, g.picker().labels())
		})

		t.Run("it should say in the title that new group… makes one", func(t *testing.T) {
			assert.Contains(t, g.picker().title(), "new group… makes one")
		})

		t.Run("and a group is chosen", func(t *testing.T) {
			g, actor := picking(t)
			press(t, g, ebiten.KeyArrowDown)
			press(t, g, ebiten.KeyArrowDown)
			press(t, g, ebiten.KeyEnter)

			t.Run("it should move the session there", func(t *testing.T) {
				assert.Equal(t, []city.Action{{Kind: city.ActionMove, SessionID: "a", Group: "g2"}}, actor.done)
			})

			t.Run("it should close the picker", func(t *testing.T) {
				assert.Nil(t, g.picker())
			})
		})

		t.Run("and no group is chosen", func(t *testing.T) {
			g, actor := picking(t)
			press(t, g, ebiten.KeyArrowDown)
			press(t, g, ebiten.KeyArrowDown)
			press(t, g, ebiten.KeyArrowDown)
			press(t, g, ebiten.KeyEnter)

			t.Run("it should take the session out of every group", func(t *testing.T) {
				assert.Equal(t, []city.Action{{Kind: city.ActionMove, SessionID: "a", Group: ""}}, actor.done)
			})
		})

		t.Run("and esc is pressed", func(t *testing.T) {
			g, actor := picking(t)
			press(t, g, ebiten.KeyEscape)

			t.Run("it should move nothing", func(t *testing.T) {
				assert.Empty(t, actor.done)
			})
		})

		t.Run("and new group… is chosen", func(t *testing.T) {
			naming := func(t *testing.T, name string) (*Game, *recordingActor) {
				t.Helper()
				g, actor := picking(t)
				press(t, g, ebiten.KeyEnter)
				for _, r := range name {
					g.scriptedRune = r
					require.NoError(t, g.handleKeys())
					g.scriptedRune = 0
				}
				return g, actor
			}
			g, _ := naming(t, "")

			t.Run("it should ask for a name in the picker", func(t *testing.T) {
				assert.True(t, g.picker().naming)
			})

			t.Run("and a name is typed and enter is pressed", func(t *testing.T) {
				g, actor := naming(t, "release")
				press(t, g, ebiten.KeyEnter)

				t.Run("it should move the session into a new group by that name", func(t *testing.T) {
					assert.Equal(t, []city.Action{{Kind: city.ActionMove, SessionID: "a", GroupName: "release"}}, actor.done)
				})

				t.Run("it should close the picker", func(t *testing.T) {
					assert.Nil(t, g.picker())
				})
			})

			refusals := []struct {
				when, name, says string
			}{
				{"and enter is pressed with no name", "", "a new group needs a name"},
				{"and enter is pressed with only spaces", "   ", "a new group needs a name"},
				{"and a name already taken is typed", " log-88 · BUFFER ", "LOG-88 · buffer is already a group"},
			}
			for _, r := range refusals {
				t.Run(r.when, func(t *testing.T) {
					g, actor := naming(t, r.name)
					press(t, g, ebiten.KeyEnter)

					t.Run("it should refuse in words in the picker", func(t *testing.T) {
						assert.Equal(t, r.says, g.picker().refused)
					})

					t.Run("it should do nothing", func(t *testing.T) {
						assert.Empty(t, actor.done)
					})
				})
			}

			t.Run("and esc is pressed while naming", func(t *testing.T) {
				g, actor := naming(t, "rel")
				press(t, g, ebiten.KeyEscape)

				t.Run("it should return to the list", func(t *testing.T) {
					require.NotNil(t, g.picker())
					assert.False(t, g.picker().naming)
				})

				t.Run("it should do nothing", func(t *testing.T) {
					assert.Empty(t, actor.done)
				})
			})
		})
	})

	t.Run("when a drag starts", func(t *testing.T) {
		g, _ := newGame()
		a := g.scene.City().Buildings()[0]
		other := g.scene.City().Buildings()[1]

		t.Run("and it is on the selected building", func(t *testing.T) {
			t.Run("it should carry the building", func(t *testing.T) {
				assert.True(t, carries(city.Hit{Building: a}, a))
			})
		})

		t.Run("and it is on another building", func(t *testing.T) {
			t.Run("it should pan as before", func(t *testing.T) {
				assert.False(t, carries(city.Hit{Building: other}, a))
			})
		})

		t.Run("and nothing is selected", func(t *testing.T) {
			t.Run("it should pan as before", func(t *testing.T) {
				assert.False(t, carries(city.Hit{Building: a}, nil))
			})
		})
	})

	t.Run("when a carried building is dropped", func(t *testing.T) {
		g, _ := newGame()
		a := g.scene.City().Buildings()[0]
		line := &city.Line{Group: &groups[1]}

		t.Run("and it lands on a line's station", func(t *testing.T) {
			t.Run("it should move the session to that line's group", func(t *testing.T) {
				assert.Equal(t, city.Action{Kind: city.ActionMove, SessionID: a.Session.ID, Group: "g2"},
					dropAction(a, city.Hit{Station: &city.StationHit{Line: line}}))
			})
		})

		t.Run("and it lands anywhere else", func(t *testing.T) {
			t.Run("it should do nothing", func(t *testing.T) {
				assert.Equal(t, city.Action{}, dropAction(a, city.Hit{}))
			})
		})
	})
}
