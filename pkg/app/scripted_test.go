package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 273: scripted runs are hidden by default, and a row shows them.
func TestScriptedRunsSetting(t *testing.T) {
	t.Run("when the config is the default", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		cfg, err := config.New(config.NewViper())
		require.NoError(t, err)
		var row ui.Setting
		for _, s := range settingsFor(cfg).Items {
			if s.Key == config.KeyScriptedRuns {
				row = s
			}
		}

		t.Run("it should show a scripted runs row reading hidden", func(t *testing.T) {
			assert.Equal(t, [2]string{"scripted runs", "hidden"}, [2]string{row.Label, row.Value})
		})

		t.Run("it should offer hidden and shown", func(t *testing.T) {
			assert.Equal(t, []string{"hidden", "shown"}, row.Options)
		})
	})

	t.Run("when the panel shows them", func(t *testing.T) {
		key, value := settingValue(ui.Setting{Key: config.KeyScriptedRuns, Value: "shown"})

		t.Run("it should save scripted_runs = shown", func(t *testing.T) {
			assert.Equal(t, []any{config.KeyScriptedRuns, "shown"}, []any{key, value})
		})
	})
}
