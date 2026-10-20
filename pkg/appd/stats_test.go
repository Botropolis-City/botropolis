package appd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/stats"
)

// Item 110: hefts come from the statistics store while statistics are
// on and nothing about them is owed, and from a whole read otherwise.
func TestStatsWiring(t *testing.T) {
	stateDir := func(t *testing.T) string {
		t.Helper()
		root := t.TempDir()
		t.Setenv("XDG_STATE_HOME", root)
		return filepath.Join(root, "botropolis")
	}

	t.Run("when stats are on and nothing is owed", func(t *testing.T) {
		stateDir(t)
		src := statsSource(&config.Config{Stats: true})
		if src != nil {
			t.Cleanup(func() { _ = src.Close() })
		}

		t.Run("it should take hefts from the store", func(t *testing.T) {
			assert.NotNil(t, src)
		})

		t.Run("and it is asked again in the same process", func(t *testing.T) {
			t.Run("it should give the same store", func(t *testing.T) {
				assert.Same(t, src, statsSource(&config.Config{Stats: true}))
			})
		})
	})

	t.Run("when stats are off", func(t *testing.T) {
		dir := stateDir(t)
		src := statsSource(&config.Config{Stats: false})

		t.Run("it should read whole, as before", func(t *testing.T) {
			assert.Nil(t, src)
		})

		t.Run("it should not create a database", func(t *testing.T) {
			assert.NoFileExists(t, filepath.Join(dir, "stats.db"))
		})
	})

	t.Run("when the daemon was pointed at a home", func(t *testing.T) {
		dir := stateDir(t)
		src := statsSource(&config.Config{Stats: true, HomeSet: true})

		t.Run("it should read whole", func(t *testing.T) {
			assert.Nil(t, src)
		})

		t.Run("it should not create a database", func(t *testing.T) {
			assert.NoFileExists(t, filepath.Join(dir, "stats.db"))
		})
	})

	t.Run("when the on prompt is owed", func(t *testing.T) {
		dir := stateDir(t)
		s, err := stats.Open(filepath.Join(dir, "stats.db"))
		require.NoError(t, err)
		require.NoError(t, s.Close())
		require.NoError(t, os.WriteFile(filepath.Join(dir, "stats-off"), nil, 0o600))
		src := statsSource(&config.Config{Stats: true})

		t.Run("it should read whole", func(t *testing.T) {
			assert.Nil(t, src)
		})

		t.Run("it should not open the store", func(t *testing.T) {
			assert.NoFileExists(t, filepath.Join(dir, "stats.db-wal"))
		})
	})
}
