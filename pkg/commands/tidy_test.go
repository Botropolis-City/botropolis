package commands_test

import (
	"errors"
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

	t.Run("when a group is closed", func(t *testing.T) {
		closing := func(t *testing.T, fails string) (group.Memory, []string, error) {
			t.Helper()
			path := filepath.Join(t.TempDir(), "groups.json")
			require.NoError(t, group.Save(path, stored))
			var packed []string
			actor := commands.Actor{GroupStore: path, Pack: func(id string, pack bool) error {
				if id == fails {
					return errors.New("no such session")
				}
				if pack {
					packed = append(packed, id)
				}
				return nil
			}}
			err := actor.Do(city.Action{Kind: city.ActionCloseGroup, Group: "g1", Sessions: []string{"a", "b", "c"}})
			memory, loadErr := group.Load(path)
			require.NoError(t, loadErr)
			return memory, packed, err
		}

		t.Run("and every session packs", func(t *testing.T) {
			memory, packed, err := closing(t, "")
			require.NoError(t, err)

			t.Run("it should pack each of its sessions", func(t *testing.T) {
				assert.Equal(t, []string{"a", "b", "c"}, packed)
			})

			t.Run("it should store the group closed", func(t *testing.T) {
				assert.Equal(t, []string{"g1"}, memory.Closed)
			})
		})

		t.Run("and a session fails to pack", func(t *testing.T) {
			memory, _, err := closing(t, "b")

			t.Run("it should say which", func(t *testing.T) {
				assert.ErrorContains(t, err, "b: no such session")
			})

			t.Run("it should not store the group closed", func(t *testing.T) {
				assert.Empty(t, memory.Closed)
			})
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
