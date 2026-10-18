package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
)

// Item 106: the top-down projection is no longer offered. A config that
// still asks for it loads, says so once, and opens the isometric map.
func TestProjectionSetting(t *testing.T) {
	t.Run("when the settings panel is built", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		cfg, err := config.New(config.NewViper())
		require.NoError(t, err)
		var keys []string
		for _, s := range settingsFor(cfg).Items {
			keys = append(keys, s.Key)
		}

		t.Run("it should offer no projection row", func(t *testing.T) {
			assert.NotContains(t, keys, config.KeyProjection)
		})
	})

	t.Run("when a config file says projection = top", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", home)
		require.NoError(t, os.MkdirAll(filepath.Join(home, "botropolis"), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(home, "botropolis", "config.toml"), []byte("projection = \"top\"\n"), 0o600))
		v := config.NewViper()
		require.NoError(t, v.ReadInConfig())
		cfg, err := config.New(v)
		require.NoError(t, err)
		var log bytes.Buffer
		p, err := projectionFor(cfg.Projection, &log)

		t.Run("it should load", func(t *testing.T) {
			assert.NoError(t, err)
		})

		t.Run("it should log one line", func(t *testing.T) {
			assert.Equal(t, 1, strings.Count(log.String(), "\n"))
		})

		t.Run("it should open isometric", func(t *testing.T) {
			assert.Equal(t, city.Isometric, p)
		})
	})

	t.Run("when the projection is unknown", func(t *testing.T) {
		_, err := projectionFor("sideways", &bytes.Buffer{})

		t.Run("it should refuse it", func(t *testing.T) {
			assert.Error(t, err)
		})
	})
}
