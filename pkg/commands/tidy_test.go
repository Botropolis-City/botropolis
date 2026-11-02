package commands_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/group"
)

// Item 333: a group renamed, or a session taken out of one, from the map.
func TestActorCorrectsGroups(t *testing.T) {
	stored := group.Memory{
		Groups: []group.Stored{{ID: "g1", Name: "PROJ-9", Members: []string{"a", "b", "c"}}},
		Muted:  []string{"headers"},
	}
	acted := func(t *testing.T, action city.Action) group.Memory {
		t.Helper()
		path := filepath.Join(t.TempDir(), "groups.json")
		require.NoError(t, group.Save(path, stored))
		require.NoError(t, commands.Actor{GroupStore: path}.Do(action))
		memory, err := group.Load(path)
		require.NoError(t, err)
		return memory
	}

	t.Run("when a group is renamed", func(t *testing.T) {
		memory := acted(t, city.Action{Kind: city.ActionRename, Group: "g1", GroupName: "The limiter"})

		t.Run("it should store the name as named by hand", func(t *testing.T) {
			assert.Equal(t, group.Stored{ID: "g1", Name: "The limiter", Renamed: true, Members: []string{"a", "b", "c"}}, memory.Groups[0])
		})

		t.Run("it should keep what memory already held", func(t *testing.T) {
			assert.Equal(t, []string{"headers"}, memory.Muted)
		})
	})

	t.Run("when a session is taken out of a group", func(t *testing.T) {
		memory := acted(t, city.Action{Kind: city.ActionTakeOut, SessionID: "c", Group: "g1"})

		t.Run("it should remember it as not related to the other members", func(t *testing.T) {
			assert.Equal(t, []group.Pair{{"c", "a"}, {"c", "b"}}, memory.NotRelated)
		})
	})

	for _, kind := range []city.ActionKind{city.ActionRename, city.ActionTakeOut} {
		t.Run("when the actor keeps no groups and is asked to "+string(kind), func(t *testing.T) {
			err := commands.Actor{}.Do(city.Action{Kind: kind, SessionID: "c", Group: "g1", GroupName: "x"})

			t.Run("it should refuse and say why", func(t *testing.T) {
				assert.ErrorContains(t, err, "groups are kept only for your own home")
			})
		})
	}
}
