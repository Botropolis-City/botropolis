package state_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/state"
)

// Item 333: the quiet sweep packs sessions up and marks them swept, so
// what it did can be counted afterwards.
func TestLedgerSweep(t *testing.T) {
	at := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	seen := state.Session{ID: "aaaaaaaa-0000-0000-0000-000000000009", CWD: loaderCWD, State: state.Resting, StartedAt: at.Add(-48 * time.Hour), LastActivity: at.Add(-24 * time.Hour)}
	swept := func() state.Ledger {
		ledger := state.Ledger{}
		ledger.Record(seen)
		ledger.Sweep(seen.ID, at)
		return ledger
	}

	t.Run("when a session is swept", func(t *testing.T) {
		t.Run("it should be packed", func(t *testing.T) {
			assert.True(t, swept()[seen.ID].Packed())
		})

		t.Run("it should be marked swept", func(t *testing.T) {
			assert.True(t, swept()[seen.ID].Swept())
		})

		t.Run("and it is seen again", func(t *testing.T) {
			ledger := swept()
			later := seen
			later.LastActivity = later.LastActivity.Add(time.Minute)
			ledger.Record(later)

			t.Run("it should stay marked swept", func(t *testing.T) {
				assert.True(t, ledger[seen.ID].Swept())
			})
		})

		for _, c := range []struct {
			name string
			act  func(state.Ledger)
		}{
			{"and it is unpacked", func(l state.Ledger) { l.Unpack(seen.ID, at.Add(time.Hour)) }},
			{"and it is packed again by hand", func(l state.Ledger) { l.Pack(seen.ID, at.Add(time.Hour)) }},
		} {
			t.Run(c.name, func(t *testing.T) {
				ledger := swept()
				c.act(ledger)

				t.Run("it should no longer be marked swept", func(t *testing.T) {
					assert.False(t, ledger[seen.ID].Swept())
				})
			})
		}

		t.Run("and the ledger is saved and loaded", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sessions.json")
			require.NoError(t, swept().Save(path))
			loaded, err := state.LoadLedger(path)
			require.NoError(t, err)

			t.Run("it should stay marked swept", func(t *testing.T) {
				assert.True(t, loaded[seen.ID].Swept())
			})
		})
	})
}
