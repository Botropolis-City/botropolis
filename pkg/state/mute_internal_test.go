package state

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/group"
)

// Item 90: a session joined on wording carries the words that joined it,
// so its card can offer to mute one.
func TestGroupWords(t *testing.T) {
	joined := func(reason group.Reason) Session {
		sessions := []Session{{ID: "a"}, {ID: "b"}}
		groupViews(group.Result{Groups: []group.Group{{ID: "g1", Members: []group.Membership{
			{Session: "a", Tier: group.Sure},
			{Session: "b", Tier: group.Likely, Reason: reason},
		}}}}, sessions)
		return sessions[1]
	}

	t.Run("when a session joined its group on wording", func(t *testing.T) {
		t.Run("it should carry the words that joined it", func(t *testing.T) {
			assert.Equal(t, []string{"kiln", "shelves"}, joined(group.Reason{Signal: group.SignalWording, Detail: "kiln shelves", With: "a"}).GroupWords)
		})
	})

	t.Run("when it joined on a reference", func(t *testing.T) {
		t.Run("it should carry none", func(t *testing.T) {
			assert.Empty(t, joined(group.Reason{Signal: group.SignalReference, Detail: "PROJ-9", With: "a"}).GroupWords)
		})
	})
}
