package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
)

// Item 350, slice 5a: a binding is set in the config file's keys table,
// or with --set keys.<name>=<chord>, and the city reads it from there.
func TestKeysFromTheConfig(t *testing.T) {
	bound := func(t *testing.T, cfg *config.Config, name string) config.Chord {
		t.Helper()
		chords, err := config.FindChords(*cfg, "linux", anyKey)
		require.NoError(t, err)
		return chords.Bound[name]
	}

	t.Run("when the config file sets a binding in its keys table", func(t *testing.T) {
		configHome := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", configHome)
		require.NoError(t, os.MkdirAll(filepath.Join(configHome, "botropolis"), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(configHome, "botropolis", "config.toml"), []byte("[keys]\ntimeline = \"y\"\n"), 0o600))
		cfg := load(t)

		t.Run("it should bind it", func(t *testing.T) {
			assert.Equal(t, config.Chord{Key: "y"}, bound(t, cfg, "timeline"))
		})

		t.Run("it should leave the rest at their defaults", func(t *testing.T) {
			assert.Equal(t, config.Chord{Key: "x"}, bound(t, cfg, "breakdown"))
		})
	})

	t.Run("when --set binds one", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		cfg := load(t, "--set", "keys.timeline=y")

		t.Run("it should bind it", func(t *testing.T) {
			assert.Equal(t, config.Chord{Key: "y"}, bound(t, cfg, "timeline"))
		})
	})

	t.Run("when only an old setting is changed", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		cfg := load(t, "--set", "journal_key=y")

		t.Run("it should still count, though keys holds every binding's default", func(t *testing.T) {
			assert.Equal(t, config.Chord{Key: "y"}, bound(t, cfg, "journal"))
		})
	})
}
