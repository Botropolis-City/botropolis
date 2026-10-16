package commands_test

import (
	"path/filepath"
	"testing"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/group"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActorMove(t *testing.T) {
	t.Run("when a session is moved to a group", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "groups.json")
		require.NoError(t, group.Save(path, group.Memory{Muted: []string{"headers"}}))
		actor := commands.Actor{GroupStore: path}
		require.NoError(t, actor.Do(city.Action{Kind: city.ActionMove, SessionID: "a", Group: "g2"}))
		memory, err := group.Load(path)
		require.NoError(t, err)

		t.Run("it should write the move into the groups' memory", func(t *testing.T) {
			assert.Equal(t, []group.Move{{Session: "a", Group: "g2"}}, memory.Moves)
		})

		t.Run("it should keep what memory already held", func(t *testing.T) {
			assert.Equal(t, []string{"headers"}, memory.Muted)
		})
	})

	t.Run("when the actor keeps no groups, as for a run pointed at another home", func(t *testing.T) {
		err := commands.Actor{}.Do(city.Action{Kind: city.ActionMove, SessionID: "a", Group: "g2"})

		t.Run("it should refuse and say why", func(t *testing.T) {
			assert.ErrorContains(t, err, "groups are kept only for your own home")
		})
	})
}
