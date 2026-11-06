package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/config"
)

// Item 347: a tower's lines run to the buildings on the map, or to the
// crates in the depot as well.
func TestTowerLinesKey(t *testing.T) {
	t.Run("when tower_lines is not set", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		cfg := load(t)

		t.Run("it should run to buildings only", func(t *testing.T) {
			assert.Equal(t, config.TowerLinesBuildings, cfg.TowerLines)
		})
	})

	for _, value := range []string{config.TowerLinesBuildings, config.TowerLinesAll} {
		t.Run("when tower_lines is "+value, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			cfg := load(t, "--set", config.KeyTowerLines+"="+value)

			t.Run("it should keep it", func(t *testing.T) {
				assert.Equal(t, value, cfg.TowerLines)
			})
		})
	}

	t.Run("when tower_lines is anything else", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		_, err := loadErr(t, "--set", config.KeyTowerLines+"=crates")

		t.Run("it should refuse it, naming the key and both values", func(t *testing.T) {
			assert.ErrorContains(t, err, "unknown tower_lines \"crates\": use buildings or all")
		})
	})

	t.Run("when the promised values are listed", func(t *testing.T) {
		t.Run("it should list both", func(t *testing.T) {
			assert.ElementsMatch(t, []string{config.TowerLinesBuildings, config.TowerLinesAll}, config.Values[config.KeyTowerLines])
		})
	})
}
