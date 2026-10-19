package state_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/statedir"
)

// Item 149: the ledger carries a version, so a ledger of another version
// is migrated forward or refused by name, and never reset.
func TestLedgerVersions(t *testing.T) {
	at := time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)
	entry := state.LedgerEntry{CWD: "/w/lib", StartedAt: at.Add(-time.Hour), LastActivity: at}
	const id = "aaaaaaaa-0000-0000-0000-000000000001"
	write := func(t *testing.T, body string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "sessions.json")
		require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
		return path
	}
	fieldsOf := func(t *testing.T, path string) map[string]json.RawMessage {
		t.Helper()
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		var fields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(data, &fields))
		return fields
	}

	t.Run("when the ledger is the bare map from before versions", func(t *testing.T) {
		loaded, err := state.LoadLedger(write(t, `{"`+id+`":{"cwd":"/w/lib","startedAt":"2026-10-19T08:00:00Z","lastActivity":"2026-10-19T09:00:00Z"}}`))
		require.NoError(t, err)

		t.Run("it should load every entry", func(t *testing.T) {
			assert.Equal(t, state.Ledger{id: entry}, loaded)
		})
	})

	t.Run("when a ledger is saved", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "sessions.json")
		require.NoError(t, state.Ledger{id: entry}.Save(path))
		fields := fieldsOf(t, path)

		for key, want := range map[string]string{"version": "2", "readableFrom": "2"} {
			t.Run("it should write "+key, func(t *testing.T) {
				assert.Equal(t, want, string(fields[key]))
			})
		}

		t.Run("it should load back equal", func(t *testing.T) {
			loaded, err := state.LoadLedger(path)
			require.NoError(t, err)
			assert.Equal(t, state.Ledger{id: entry}, loaded)
		})
	})

	t.Run("when the ledger is from a newer version", func(t *testing.T) {
		t.Run("and this build can read it", func(t *testing.T) {
			path := write(t, `{"version":7,"readableFrom":2,"sessions":{"`+id+`":{"cwd":"/w/lib","startedAt":"2026-10-19T08:00:00Z","lastActivity":"2026-10-19T09:00:00Z"}}}`)
			loaded, err := state.LoadLedger(path)
			require.NoError(t, err)

			t.Run("it should load every entry", func(t *testing.T) {
				assert.Equal(t, state.Ledger{id: entry}, loaded)
			})

			t.Run("and it is saved", func(t *testing.T) {
				require.NoError(t, loaded.Save(path))
				fields := fieldsOf(t, path)

				for key, want := range map[string]string{"version": "7", "readableFrom": "2"} {
					t.Run("it should keep the newer "+key, func(t *testing.T) {
						assert.Equal(t, want, string(fields[key]))
					})
				}
			})
		})

		t.Run("and this build cannot read it", func(t *testing.T) {
			path := write(t, `{"version":7,"readableFrom":7,"sessions":{}}`)
			_, err := state.LoadLedger(path)
			require.Error(t, err)

			t.Run("it should be a newer-file error", func(t *testing.T) {
				assert.True(t, errors.Is(err, statedir.ErrNewer))
			})

			t.Run("it should name the file and both versions", func(t *testing.T) {
				assert.Contains(t, err.Error(), path+" is ledger version 7, which this build's version 2 cannot read")
			})
		})

		t.Run("and this build cannot read it, and the loader loads and packs a session", func(t *testing.T) {
			body := `{"version":7,"readableFrom":7,"sessions":{}}`
			path := write(t, body)
			loader := state.NewLoader(liveHome(t).Path, alive).WithLedger(path)
			_, _ = loader.Load(now)
			_ = loader.Pack(sidA, true, now)
			_, _ = loader.Load(now)
			after, err := os.ReadFile(path)
			require.NoError(t, err)

			t.Run("it should leave the file as it was", func(t *testing.T) {
				assert.Equal(t, body, string(after))
			})
		})
	})
}
