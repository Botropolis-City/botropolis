package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/config"
)

// Item 348: a group's line runs between its districts by default, or
// outlines its neighbourhoods and links them, or runs session to session
// (Aria, 2026-11-06).
func TestGroupLineFormKey(t *testing.T) {
	forms := []string{config.GroupLineDistricts, config.GroupLineNeighbourhoods, config.GroupLineSessions}

	t.Run("when group_line_form is not set", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		cfg := load(t)

		t.Run("it should run between districts", func(t *testing.T) {
			assert.Equal(t, config.GroupLineDistricts, cfg.GroupLineForm)
		})
	})

	for _, value := range forms {
		t.Run("when group_line_form is "+value, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			cfg := load(t, "--set", config.KeyGroupLineForm+"="+value)

			t.Run("it should keep it", func(t *testing.T) {
				assert.Equal(t, value, cfg.GroupLineForm)
			})
		})
	}

	t.Run("when group_line_form is anything else", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		_, err := loadErr(t, "--set", config.KeyGroupLineForm+"=stations")

		t.Run("it should refuse it, naming the key and the three values", func(t *testing.T) {
			assert.ErrorContains(t, err, "unknown group_line_form \"stations\": use districts, neighbourhoods or sessions")
		})
	})

	t.Run("when the promised values are listed", func(t *testing.T) {
		t.Run("it should list all three", func(t *testing.T) {
			assert.ElementsMatch(t, forms, config.Values[config.KeyGroupLineForm])
		})
	})
}
