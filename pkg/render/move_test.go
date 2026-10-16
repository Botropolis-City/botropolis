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

		t.Run("it should offer every group and no group", func(t *testing.T) {
			assert.Equal(t, []string{"PROJ-123 · headers", "LOG-88 · buffer", "no group"}, g.picker().labels())
		})

		t.Run("and a group is chosen", func(t *testing.T) {
			g, actor := picking(t)
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
