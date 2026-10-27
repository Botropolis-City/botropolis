package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/config"
)

// Item 293: closing the window asks, unless the setting sends the city
// to the tray.
func TestOnCloseKey(t *testing.T) {
	t.Run("when on_close is not set", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		cfg := load(t)

		t.Run("it should ask", func(t *testing.T) {
			assert.Equal(t, config.OnCloseAsk, cfg.OnClose)
		})
	})

	for _, value := range []string{config.OnCloseAsk, config.OnCloseTray} {
		t.Run("when on_close is "+value, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			cfg := load(t, "--set", config.KeyOnClose+"="+value)

			t.Run("it should keep it", func(t *testing.T) {
				assert.Equal(t, value, cfg.OnClose)
			})
		})
	}

	t.Run("when on_close is anything else", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		_, err := loadErr(t, "--set", config.KeyOnClose+"=minimise")

		t.Run("it should refuse it, naming both values", func(t *testing.T) {
			assert.ErrorContains(t, err, "use ask or tray")
		})
	})
}
