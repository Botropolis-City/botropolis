package state_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/group"
	"botropolis.city/botropolis/pkg/state"
)

// Item 348: a session says whether its group, or none, is a move by hand,
// so the city moves only the buildings Aria changed.
func TestLoaderGroupMoved(t *testing.T) {
	load := func(t *testing.T, memory group.Memory) state.Snapshot {
		t.Helper()
		home := groupedHome(t)
		path := filepath.Join(t.TempDir(), "groups.json")
		require.NoError(t, group.Save(path, memory))
		snapshot, err := state.NewLoader(home.Path, dead).WithGroups(path).Load(now)
		require.NoError(t, err)
		return snapshot
	}

	t.Run("when a session was moved to none by hand", func(t *testing.T) {
		snapshot := load(t, group.Memory{}.WithMove(sidB, ""))

		t.Run("it should mark the moved session", func(t *testing.T) {
			assert.True(t, sessionByID(t, snapshot, sidB).GroupMoved)
		})

		t.Run("it should not mark the other", func(t *testing.T) {
			assert.False(t, sessionByID(t, snapshot, sidA).GroupMoved)
		})
	})

	t.Run("when a session was moved into a group by hand", func(t *testing.T) {
		memory := group.Memory{Groups: []group.Stored{{ID: "g-new", Name: "Gateway", Renamed: true}}}.WithMove(sidB, "g-new")
		snapshot := load(t, memory)

		t.Run("it should mark the moved session", func(t *testing.T) {
			assert.True(t, sessionByID(t, snapshot, sidB).GroupMoved)
		})
	})

	t.Run("when the sessions joined on what they said", func(t *testing.T) {
		snapshot := load(t, group.Memory{})

		t.Run("it should mark neither", func(t *testing.T) {
			assert.Equal(t, []bool{false, false}, []bool{sessionByID(t, snapshot, sidA).GroupMoved, sessionByID(t, snapshot, sidB).GroupMoved})
		})
	})
}
