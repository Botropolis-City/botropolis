package config_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/config"
)

// Item 273: scripted runs are hidden unless the setting shows them.
func TestScriptedRunsKey(t *testing.T) {
	t.Run("when scripted_runs is not set", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		cfg := load(t)

		t.Run("it should hide them", func(t *testing.T) {
			assert.False(t, cfg.ScriptedRuns)
		})
	})

	for _, c := range []struct {
		value string
		want  bool
	}{{config.ScriptedShown, true}, {config.ScriptedHidden, false}} {
		t.Run("when scripted_runs is "+c.value, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			cfg := load(t, "--set", config.KeyScriptedRuns+"="+c.value)

			t.Run(fmt.Sprintf("it should show them is %v", c.want), func(t *testing.T) {
				assert.Equal(t, c.want, cfg.ScriptedRuns)
			})
		})
	}

	t.Run("when scripted_runs is anything else", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		_, err := loadErr(t, "--set", config.KeyScriptedRuns+"=sometimes")

		t.Run("it should refuse it", func(t *testing.T) {
			assert.Error(t, err)
		})
	})
}
