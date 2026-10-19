package state_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
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
		changed := ledger.Forget(demolished, at)

		t.Run("it should be marked demolished", func(t *testing.T) {
			assert.True(t, ledger[demolished].Demolished())
		})

		t.Run("it should say the ledger changed", func(t *testing.T) {
			assert.True(t, changed)
		})

		t.Run("and it is forgotten again", func(t *testing.T) {
			t.Run("it should say nothing changed", func(t *testing.T) {
				assert.False(t, ledger.Forget(demolished, at))
			})
		})
	})

	t.Run("when an unknown session is forgotten", func(t *testing.T) {
		ledger := state.Ledger{demolished: entry}
		ledger.Forget(cleaned, at)

		t.Run("it should be marked demolished", func(t *testing.T) {
			assert.True(t, ledger[cleaned].Demolished())
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

		t.Run("it should keep its tombstone in the saved ledger", func(t *testing.T) {
			assert.True(t, saved[demolished].Demolished())
		})

		t.Run("and another session's transcript was deleted by Claude Code", func(t *testing.T) {
			t.Run("it should still show it as gone", func(t *testing.T) {
				assert.True(t, sessionByID(t, snapshot, cleaned).ConversationGone)
			})
		})
	})

	t.Run("when the loader forgets a demolished session whose transcript remains", func(t *testing.T) {
		path := ledgerAt(t)
		home := liveHome(t)
		loader := state.NewLoader(home.Path, alive).WithLedger(path)
		first, err := loader.Load(now)
		require.NoError(t, err)
		require.True(t, shown(first, otherSid))
		require.NoError(t, loader.Forget(otherSid))
		next, err := loader.Load(now.Add(time.Second))
		require.NoError(t, err)
		after, err := loader.Load(now.Add(2 * time.Second))
		require.NoError(t, err)

		t.Run("it should not be in the next load", func(t *testing.T) {
			assert.False(t, shown(next, otherSid))
		})

		t.Run("it should not be in the load after that", func(t *testing.T) {
			assert.False(t, shown(after, otherSid))
		})

		t.Run("it should keep the session that is still there", func(t *testing.T) {
			assert.True(t, shown(after, sidA))
		})

		t.Run("and a new loader starts on the same ledger", func(t *testing.T) {
			restarted, err := state.NewLoader(home.Path, alive).WithLedger(path).Load(now.Add(3 * time.Second))
			require.NoError(t, err)

			t.Run("it should not be in its first load", func(t *testing.T) {
				assert.False(t, shown(restarted, otherSid))
			})
		})
	})

	t.Run("when a demolished session whose transcript remains is resumed", func(t *testing.T) {
		path := ledgerAt(t)
		home := liveHome(t)
		loader := state.NewLoader(home.Path, alive).WithLedger(path)
		require.NoError(t, loader.Forget(otherSid))
		resumedAt := time.Now().UTC().Add(time.Hour)
		home.AppendTranscript(otherSid, loaderCWD,
			helpers.UserPrompt(otherSid, loaderCWD, resumedAt.Format(time.RFC3339Nano)))
		snapshot, err := loader.Load(resumedAt.Add(time.Minute))
		require.NoError(t, err)
		saved, err := state.LoadLedger(path)
		require.NoError(t, err)

		t.Run("it should be back in the next load", func(t *testing.T) {
			assert.True(t, shown(snapshot, otherSid))
		})

		t.Run("it should drop its tombstone from the saved ledger", func(t *testing.T) {
			assert.False(t, saved[otherSid].Demolished())
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

			t.Run("it should mark it demolished in the saved ledger", func(t *testing.T) {
				saved, err := state.LoadLedger(path)
				require.NoError(t, err)
				assert.True(t, saved[demolished].Demolished())
			})

			t.Run("it should clear the notes it applied", func(t *testing.T) {
				_, err := os.Stat(state.ForgottenPath(path))
				assert.ErrorIs(t, err, os.ErrNotExist)
			})
		})
	})

	t.Run("when a demolish of a session whose transcript remains is noted while no daemon runs", func(t *testing.T) {
		path := ledgerAt(t)
		require.NoError(t, state.NoteForgotten(path, otherSid))

		t.Run("and a loader then starts on the ledger", func(t *testing.T) {
			snapshot, err := state.NewLoader(liveHome(t).Path, alive).WithLedger(path).Load(now)
			require.NoError(t, err)

			t.Run("it should not show it in its first load", func(t *testing.T) {
				assert.False(t, shown(snapshot, otherSid))
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
