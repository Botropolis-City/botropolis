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

func TestLedger(t *testing.T) {
	at := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	seen := state.Session{
		ID: "aaaaaaaa-0000-0000-0000-000000000001", CWD: loaderCWD, State: state.Working,
		Title: "Fix the CI queue", LastPrompt: "fix it", StartedAt: at.Add(-time.Hour), LastActivity: at,
	}

	t.Run("when a session is seen", func(t *testing.T) {
		ledger := state.Ledger{}
		changed := ledger.Record(seen)

		t.Run("it should record its working directory and times", func(t *testing.T) {
			assert.Equal(t, state.LedgerEntry{CWD: loaderCWD, StartedAt: at.Add(-time.Hour), LastActivity: at}, ledger[seen.ID])
		})

		t.Run("it should say the ledger changed", func(t *testing.T) {
			assert.True(t, changed)
		})

		t.Run("and it is seen again unchanged", func(t *testing.T) {
			t.Run("it should say nothing changed", func(t *testing.T) {
				assert.False(t, ledger.Record(seen))
			})
		})
	})

	t.Run("when an empty session is seen", func(t *testing.T) {
		ledger := state.Ledger{}
		empty := seen
		empty.State = state.Empty
		ledger.Record(empty)

		t.Run("it should not record it, because prune clears it on purpose", func(t *testing.T) {
			assert.Empty(t, ledger)
		})
	})

	t.Run("when it is saved and loaded", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "botropolis", "sessions.json")
		ledger := state.Ledger{}
		ledger.Record(seen)
		require.NoError(t, ledger.Save(path))
		loaded, err := state.LoadLedger(path)
		require.NoError(t, err)

		t.Run("it should round-trip", func(t *testing.T) {
			assert.Equal(t, ledger, loaded)
		})

		t.Run("it should hold no title or prompt, only ids, directories and times", func(t *testing.T) {
			data, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.NotContains(t, string(data), "fix it")
		})
	})

	t.Run("when a session is packed up", func(t *testing.T) {
		ledger := state.Ledger{}
		ledger.Record(seen)
		ledger.Pack(seen.ID, at.Add(time.Hour))

		t.Run("it should be packed", func(t *testing.T) {
			assert.True(t, ledger[seen.ID].Packed())
		})

		t.Run("it should record when", func(t *testing.T) {
			assert.Equal(t, at.Add(time.Hour), ledger[seen.ID].PackedAt)
		})

		t.Run("and it is seen again", func(t *testing.T) {
			ledger.Record(seen)

			t.Run("it should stay packed", func(t *testing.T) {
				assert.True(t, ledger[seen.ID].Packed())
			})
		})

		t.Run("and it is unpacked", func(t *testing.T) {
			ledger.Unpack(seen.ID, at.Add(2*time.Hour))

			t.Run("it should not be packed", func(t *testing.T) {
				assert.False(t, ledger[seen.ID].Packed())
			})

			t.Run("it should record the unpack", func(t *testing.T) {
				assert.Equal(t, at.Add(2*time.Hour), ledger[seen.ID].UnpackedAt)
			})

			t.Run("and it is saved and loaded", func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "sessions.json")
				require.NoError(t, ledger.Save(path))
				loaded, err := state.LoadLedger(path)
				require.NoError(t, err)

				t.Run("it should keep both times", func(t *testing.T) {
					assert.Equal(t, ledger[seen.ID], loaded[seen.ID])
				})
			})
		})
	})

	t.Run("when the file is missing", func(t *testing.T) {
		loaded, err := state.LoadLedger(filepath.Join(t.TempDir(), "none.json"))

		t.Run("it should load empty", func(t *testing.T) {
			assert.Empty(t, loaded)
		})

		t.Run("it should not fail", func(t *testing.T) {
			assert.NoError(t, err)
		})
	})
}
