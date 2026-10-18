package app

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/stats"
)

// Item 110: prune forgets what it removes only while statistics are on.
func TestPruneForgetter(t *testing.T) {
	const session = "11111111-0000-0000-0000-000000000000"

	t.Run("when stats are off", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", t.TempDir())

		t.Run("it should give prune nothing to forget with", func(t *testing.T) {
			assert.Nil(t, pruneForgetter(&config.Config{Stats: false}))
		})
	})

	t.Run("when stats are on, and the store holds the session", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", t.TempDir())
		s, err := stats.Open(stats.Path())
		require.NoError(t, err)
		_, err = s.DB().Exec("INSERT INTO sessions (id) VALUES (?)", session)
		require.NoError(t, err)
		require.NoError(t, s.Close())
		require.NoError(t, pruneForgetter(&config.Config{Stats: true})(session))
		s, err = stats.Open(stats.Path())
		require.NoError(t, err)
		t.Cleanup(func() { _ = s.Close() })
		var n int
		require.NoError(t, s.DB().QueryRow("SELECT count(*) FROM sessions").Scan(&n))

		t.Run("it should forget it", func(t *testing.T) {
			assert.Equal(t, 0, n)
		})
	})

	t.Run("when stats are on, and there is no database yet", func(t *testing.T) {
		state := t.TempDir()
		t.Setenv("XDG_STATE_HOME", state)
		require.NoError(t, pruneForgetter(&config.Config{Stats: true})(session))

		t.Run("it should not create one", func(t *testing.T) {
			assert.NoFileExists(t, filepath.Join(state, "botropolis", "stats.db"))
		})
	})
}
