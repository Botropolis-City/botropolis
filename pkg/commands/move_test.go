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

	t.Run("when a session is moved into a new group by name", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "groups.json")
		require.NoError(t, group.Save(path, group.Memory{Muted: []string{"headers"}}))
		actor := commands.Actor{GroupStore: path}
		require.NoError(t, actor.Do(city.Action{Kind: city.ActionMove, SessionID: "a", GroupName: "release"}))
		memory, err := group.Load(path)
		require.NoError(t, err)
		require.Len(t, memory.Groups, 1)
		made := memory.Groups[0]

		t.Run("it should store the group under the name typed, as named by hand", func(t *testing.T) {
			assert.Equal(t, group.Stored{ID: made.ID, Name: "release", Renamed: true}, made)
		})

		t.Run("it should write the move into that group", func(t *testing.T) {
			assert.Equal(t, []group.Move{{Session: "a", Group: made.ID}}, memory.Moves)
		})

		t.Run("it should keep what memory already held", func(t *testing.T) {
			assert.Equal(t, []string{"headers"}, memory.Muted)
		})

		t.Run("and the city groups again", func(t *testing.T) {
			result := group.Detect([]group.Member{{ID: "a", Dir: "/w/lib"}, {ID: "b", Dir: "/w/api"}}, memory)

			t.Run("it should list the new group with the session in it", func(t *testing.T) {
				var found []string
				for _, g := range result.Groups {
					if g.Name == "release" {
						for _, m := range g.Members {
							found = append(found, m.Session)
						}
					}
				}
				assert.Equal(t, []string{"a"}, found)
			})
		})
	})

	t.Run("when the actor keeps no groups, as for a run pointed at another home", func(t *testing.T) {
		err := commands.Actor{}.Do(city.Action{Kind: city.ActionMove, SessionID: "a", Group: "g2"})

		t.Run("it should refuse and say why", func(t *testing.T) {
			assert.ErrorContains(t, err, "groups are kept only for your own home")
		})
	})
}
