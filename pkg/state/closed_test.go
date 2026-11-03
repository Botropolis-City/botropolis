package state_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/group"
	"botropolis.city/botropolis/pkg/state"
)

// Item 333: a group a tidy-up walk settled says so in the snapshot.
func TestLoaderReviewedGroups(t *testing.T) {
	for _, c := range []struct {
		name    string
		settled bool
	}{
		{"when a walk settled the group", true},
		{"when no walk has settled it", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			home := groupedHome(t)
			store := filepath.Join(t.TempDir(), "groups.json")
			loader := state.NewLoader(home.Path, dead).WithGroups(store)
			first, err := loader.Load(now)
			require.NoError(t, err)
			require.Len(t, first.Groups, 1)
			if c.settled {
				memory, err := group.Load(store)
				require.NoError(t, err)
				memory.ReviewedGroups = []string{first.Groups[0].ID}
				require.NoError(t, group.Save(store, memory))
			}
			snapshot, err := loader.Load(now.Add(time.Minute))
			require.NoError(t, err)

			t.Run("it should say whether the group is settled", func(t *testing.T) {
				require.Len(t, snapshot.Groups, 1)
				assert.Equal(t, c.settled, snapshot.Groups[0].Reviewed)
			})
		})
	}
}

// Item 333: a closed group stays closed while its sessions are packed,
// and re-forms once one of them is unpacked.
func TestLoaderClosedGroups(t *testing.T) {
	closed := func(t *testing.T) (*state.Loader, string, string) {
		t.Helper()
		home := groupedHome(t)
		dir := t.TempDir()
		store := filepath.Join(dir, "groups.json")
		loader := state.NewLoader(home.Path, dead).WithGroups(store).WithLedger(filepath.Join(dir, "ledger.json"))
		first, err := loader.Load(now)
		require.NoError(t, err)
		require.Len(t, first.Groups, 1)
		id := first.Groups[0].ID
		for _, sid := range []string{sidA, sidB} {
			require.NoError(t, loader.Pack(sid, true, now.Add(time.Minute)))
		}
		memory, err := group.Load(store)
		require.NoError(t, err)
		require.NoError(t, group.Save(store, memory.WithClosed(id)))
		return loader, store, id
	}
	closedIn := func(t *testing.T, store string) []string {
		t.Helper()
		memory, err := group.Load(store)
		require.NoError(t, err)
		return memory.Closed
	}

	t.Run("when a closed group's sessions are all packed", func(t *testing.T) {
		loader, store, id := closed(t)
		snapshot, err := loader.Load(now.Add(2 * time.Minute))
		require.NoError(t, err)

		t.Run("it should not show the group", func(t *testing.T) {
			assert.Empty(t, snapshot.Groups)
		})

		t.Run("it should keep it closed", func(t *testing.T) {
			assert.Equal(t, []string{id}, closedIn(t, store))
		})

		t.Run("and one of them is unpacked", func(t *testing.T) {
			require.NoError(t, loader.Pack(sidA, false, now.Add(3*time.Minute)))
			snapshot, err := loader.Load(now.Add(4 * time.Minute))
			require.NoError(t, err)

			t.Run("it should form the group again", func(t *testing.T) {
				require.Len(t, snapshot.Groups, 1)
				assert.Equal(t, id, snapshot.Groups[0].ID)
			})

			t.Run("it should no longer hold it closed", func(t *testing.T) {
				assert.Empty(t, closedIn(t, store))
			})
		})
	})
}
