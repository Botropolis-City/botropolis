package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/config"
)

// Item 308: the light can be pinned at start, so a soak's light does not
// follow the box's clock.
func TestLightKey(t *testing.T) {
	t.Run("when light is not set", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		cfg := load(t)

		t.Run("it should be live", func(t *testing.T) {
			assert.Equal(t, config.LightLive, cfg.Light)
		})
	})

	for _, value := range []string{config.LightLive, config.LightNight, config.LightDay} {
		t.Run("when light is "+value, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			cfg := load(t, "--set", config.KeyLight+"="+value)

			t.Run("it should keep it", func(t *testing.T) {
				assert.Equal(t, value, cfg.Light)
			})
		})
	}

	t.Run("when light is anything else", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		_, err := loadErr(t, "--set", config.KeyLight+"=dusk")

		t.Run("it should refuse it, naming every value", func(t *testing.T) {
			assert.ErrorContains(t, err, "use live, night or day")
		})
	})
}
