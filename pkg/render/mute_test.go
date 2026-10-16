package render

import (
	"errors"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

type refusing struct{}

func (refusing) Do(city.Action) error { return errors.New("groups are not kept here") }

// Item 90: mute a word lists the words that joined the session, and
// mutes the one chosen.
func TestMutePicker(t *testing.T) {
	opened := func(t *testing.T) (*Game, *recorder) {
		t.Helper()
		actor := &recorder{}
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		s.SetSnapshot(state.Snapshot{
			Groups:   []state.Group{{ID: "g1", Name: "PROJ-1", Members: []string{"a"}}},
			Sessions: []state.Session{{ID: "a", CWD: "/w/lib", State: state.Working, Group: "g1", GroupWords: []string{"kiln", "shelves"}}}})
		require.True(t, s.Select("a"))
		g := &Game{scene: s, actor: actor, scripted: -1, clicked: -1}
		action, _ := s.Act(city.ActionMuteWord)
		g.act(action)
		require.NotNil(t, g.mutePicker(), "the picker should be open")
		return g, actor
	}

	t.Run("when mute a word is chosen", func(t *testing.T) {
		g, _ := opened(t)

		t.Run("it should list the words that joined it", func(t *testing.T) {
			assert.Equal(t, []string{"kiln", "shelves"}, g.mutePicker().words)
		})

		t.Run("and down then Enter is pressed", func(t *testing.T) {
			g, actor := opened(t)
			pressKey(t, g, ebiten.KeyArrowDown, false)
			pressKey(t, g, ebiten.KeyEnter, false)

			t.Run("it should ask to mute the chosen word", func(t *testing.T) {
				assert.Equal(t, []city.Action{{Kind: city.ActionMute, Text: "shelves"}}, actor.done)
			})

			t.Run("it should close", func(t *testing.T) {
				assert.Nil(t, g.mutePicker())
			})

			t.Run("the status should name the word and say the group recomputes without it", func(t *testing.T) {
				assert.Equal(t, "muted “shelves”: groups again without it", g.notice.Text(timeNow()))
			})
		})

		t.Run("and the actor refuses", func(t *testing.T) {
			g, _ := opened(t)
			g.actor = refusing{}
			pressKey(t, g, ebiten.KeyEnter, false)

			t.Run("the status should give its reason, not claim the word was muted", func(t *testing.T) {
				assert.Equal(t, "groups are not kept here", g.notice.Text(timeNow()))
			})
		})

		t.Run("and Esc is pressed", func(t *testing.T) {
			g, actor := opened(t)
			pressKey(t, g, ebiten.KeyEscape, false)

			t.Run("it should mute nothing", func(t *testing.T) {
				assert.Empty(t, actor.done)
			})
		})
	})
}
