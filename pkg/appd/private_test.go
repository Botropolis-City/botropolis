package appd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/statedir"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 111: a private run keeps its ledger in its private state
// directory, so a frame can show a session whose transcript is gone from
// a ledger seeded into the home it was pointed at.
func TestPrivateRunsKeepTheirLedger(t *testing.T) {
	now := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
	dead := state.Probes{Alive: func(int) bool { return false }, Attached: func(string) bool { return false }}
	gone := "99999999-0000-0000-0000-000000000000"
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("TMPDIR", t.TempDir())

	home := helpers.NewHome(t)
	seed := filepath.Join(home.Path, ".local", "state", "botropolis")
	require.NoError(t, os.MkdirAll(seed, 0o700))
	ledger := `{"` + gone + `":{"cwd":"/w/ruins","startedAt":"2026-10-01T09:00:00Z","lastActivity":"2026-10-01T10:00:00Z"}}`
	require.NoError(t, os.WriteFile(filepath.Join(seed, "sessions.json"), []byte(ledger), 0o600))

	load := func(t *testing.T, private bool) bool {
		t.Helper()
		if private {
			cleanup, err := statedir.MakePrivate(seed)
			require.NoError(t, err)
			defer func() { _ = cleanup() }()
		}
		snapshot, err := Harnesses(&config.Config{Home: home.Path, HomeSet: true}, dead).Load(now)
		require.NoError(t, err)
		for _, s := range snapshot.Sessions {
			if s.ID == gone && s.ConversationGone {
				return true
			}
		}
		return false
	}

	t.Run("when the state is private, and its seeded ledger holds a session whose transcript is gone", func(t *testing.T) {
		shown := load(t, true)

		t.Run("it should load that session from the direct harness", func(t *testing.T) {
			assert.True(t, shown)
		})
	})

	t.Run("when the state is not private, and the home holds the same ledger", func(t *testing.T) {
		shown := load(t, false)

		t.Run("it should keep no ledger", func(t *testing.T) {
			assert.False(t, shown)
		})
	})
}
