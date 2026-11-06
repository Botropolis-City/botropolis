package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
)

// Item 350, slice 5b: a key the settings put back to its default leaves
// the config file, rather than staying in it as an empty line that reads
// like a setting.
func TestDelete(t *testing.T) {
	staged := func(t *testing.T, content string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "config.toml")
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
		return path
	}
	read := func(t *testing.T, path string) string {
		t.Helper()
		b, err := os.ReadFile(path)
		require.NoError(t, err)
		return string(b)
	}

	t.Run("when an old key is deleted beside others", func(t *testing.T) {
		path := staged(t, "journal_key = \"y\"\nterminal = \"kitty\"\n")
		require.NoError(t, config.Delete(path, config.KeyJournal))
		text := read(t, path)

		t.Run("it should no longer hold it", func(t *testing.T) {
			assert.NotContains(t, text, "journal_key")
		})

		t.Run("it should keep the others", func(t *testing.T) {
			assert.Contains(t, text, "kitty")
		})
	})

	t.Run("when one of two keys in keys is deleted", func(t *testing.T) {
		path := staged(t, "[keys]\ntimeline = \"y\"\nbreakdown = \"k\"\n")
		require.NoError(t, config.Delete(path, config.KeySettings+".timeline"))
		text := read(t, path)

		t.Run("it should no longer hold it", func(t *testing.T) {
			assert.NotContains(t, text, "timeline")
		})

		t.Run("it should keep the other", func(t *testing.T) {
			assert.Contains(t, text, "breakdown")
		})
	})

	t.Run("when the last key in keys is deleted", func(t *testing.T) {
		path := staged(t, "terminal = \"kitty\"\n\n[keys]\ntimeline = \"y\"\n")
		require.NoError(t, config.Delete(path, config.KeySettings+".timeline"))
		text := read(t, path)

		t.Run("it should take the empty table with it", func(t *testing.T) {
			assert.NotContains(t, text, "[keys]")
		})

		t.Run("it should keep the rest", func(t *testing.T) {
			assert.Contains(t, text, "kitty")
		})
	})

	t.Run("when the key is not in the file", func(t *testing.T) {
		path := staged(t, "terminal = \"kitty\"\n")
		err := config.Delete(path, config.KeyJournal)

		t.Run("it should leave the file as it was", func(t *testing.T) {
			require.NoError(t, err)
			assert.Contains(t, read(t, path), "kitty")
		})
	})

	t.Run("when a binding with an old setting is saved", func(t *testing.T) {
		path := staged(t, "journal_key = \"y\"\nterminal = \"kitty\"\n")
		require.NoError(t, config.SaveBinding(path, "journal", "k"))
		text := read(t, path)

		t.Run("it should drop the old setting it replaces", func(t *testing.T) {
			assert.NotContains(t, text, "journal_key")
		})

		t.Run("it should hold the binding in keys", func(t *testing.T) {
			assert.Contains(t, text, "journal = 'k'")
		})
	})

	t.Run("when a binding is put back to its default", func(t *testing.T) {
		path := staged(t, "journal_key = \"y\"\n\n[keys]\njournal = \"k\"\ntimeline = \"y\"\n")
		require.NoError(t, config.SaveBinding(path, "journal", ""))
		text := read(t, path)

		t.Run("it should hold no line for it", func(t *testing.T) {
			assert.NotContains(t, text, "journal")
		})

		t.Run("it should keep the other bindings", func(t *testing.T) {
			assert.Contains(t, text, "timeline")
		})
	})

	t.Run("when there is no file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.toml")
		err := config.Delete(path, config.KeyJournal)

		t.Run("it should do nothing, and say nothing is wrong", func(t *testing.T) {
			require.NoError(t, err)
			assert.NoFileExists(t, path)
		})
	})
}
