package state_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/state"
)

// Item 144: a session you demolish is gone from the ledger, so it does
// not come back as ruins; one Claude Code cleaned up keeps its lot.
func TestLedgerForget(t *testing.T) {
	demolished := "dddddddd-0000-0000-0000-000000000004"
	cleaned := "eeeeeeee-0000-0000-0000-000000000005"
	at := time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)
	entry := state.LedgerEntry{CWD: "/where/it/was", StartedAt: at, LastActivity: at.Add(time.Hour)}
	ledgerAt := func(t *testing.T) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "sessions.json")
		require.NoError(t, state.Ledger{demolished: entry, cleaned: entry}.Save(path))
		return path
	}
	shown := func(snapshot state.Snapshot, id string) bool {
		for _, s := range snapshot.Sessions {
			if s.ID == id {
				return true
			}
		}
		return false
	}

	t.Run("when a recorded session is forgotten", func(t *testing.T) {
		ledger := state.Ledger{demolished: entry}
		changed := ledger.Forget(demolished)

		t.Run("it should not be in the ledger", func(t *testing.T) {
			assert.NotContains(t, ledger, demolished)
		})

		t.Run("it should say the ledger changed", func(t *testing.T) {
			assert.True(t, changed)
		})
	})

	t.Run("when an unknown session is forgotten", func(t *testing.T) {
		t.Run("it should say nothing changed", func(t *testing.T) {
			assert.False(t, state.Ledger{demolished: entry}.Forget(cleaned))
		})
	})

	t.Run("when the loader forgets a demolished session", func(t *testing.T) {
		path := ledgerAt(t)
		loader := state.NewLoader(liveHome(t).Path, alive).WithLedger(path)
		_, err := loader.Load(now)
		require.NoError(t, err)
		require.NoError(t, loader.Forget(demolished))
		snapshot, err := loader.Load(now)
		require.NoError(t, err)
		saved, err := state.LoadLedger(path)
		require.NoError(t, err)

		t.Run("it should not be in the next load", func(t *testing.T) {
			assert.False(t, shown(snapshot, demolished))
		})

		t.Run("it should not be in the saved ledger", func(t *testing.T) {
			assert.NotContains(t, saved, demolished)
		})

		t.Run("and another session's transcript was deleted by Claude Code", func(t *testing.T) {
			t.Run("it should still show it as gone", func(t *testing.T) {
				assert.True(t, sessionByID(t, snapshot, cleaned).ConversationGone)
			})
		})
	})

	t.Run("when the loader keeps no ledger", func(t *testing.T) {
		t.Run("it should forget nothing, without an error", func(t *testing.T) {
			assert.NoError(t, state.NewLoader(liveHome(t).Path, alive).Forget(demolished))
		})
	})

	t.Run("when a demolish is noted while no daemon runs", func(t *testing.T) {
		path := ledgerAt(t)
		require.NoError(t, state.NoteForgotten(path, demolished))

		t.Run("and a loader then starts on the ledger", func(t *testing.T) {
			snapshot, err := state.NewLoader(liveHome(t).Path, alive).WithLedger(path).Load(now)
			require.NoError(t, err)

			t.Run("it should not show it in its first load", func(t *testing.T) {
				assert.False(t, shown(snapshot, demolished))
			})

			t.Run("it should still show the one Claude Code cleaned up", func(t *testing.T) {
				assert.True(t, shown(snapshot, cleaned))
			})

			t.Run("it should drop it from the saved ledger", func(t *testing.T) {
				saved, err := state.LoadLedger(path)
				require.NoError(t, err)
				assert.NotContains(t, saved, demolished)
			})

			t.Run("it should clear the notes it applied", func(t *testing.T) {
				_, err := os.Stat(state.ForgottenPath(path))
				assert.ErrorIs(t, err, os.ErrNotExist)
			})
		})
	})

	t.Run("when a demolish is noted and there is no ledger", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "sessions.json")
		require.NoError(t, state.NoteForgotten(path, demolished))

		t.Run("it should write nothing, since nothing remembers it", func(t *testing.T) {
			entries, err := os.ReadDir(filepath.Dir(path))
			require.NoError(t, err)
			assert.Empty(t, entries)
		})
	})
}
