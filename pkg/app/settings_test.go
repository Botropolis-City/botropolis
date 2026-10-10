package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 79: how often the city fetches usage is a row in the settings
// panel, starting from manual.
func TestUsageRefreshSetting(t *testing.T) {
	row := func(t *testing.T, cfg *config.Config) ui.Setting {
		t.Helper()
		for _, s := range settingsFor(cfg).Items {
			if s.Key == config.KeyUsageRefresh {
				return s
			}
		}
		require.FailNow(t, "no usage refresh row")
		return ui.Setting{}
	}

	t.Run("when the config leaves it to the key", func(t *testing.T) {
		s := row(t, &config.Config{UsageRefresh: config.UsageManual})

		t.Run("it should show manual", func(t *testing.T) {
			assert.Equal(t, config.UsageManual, s.Value)
		})

		t.Run("it should offer manual first and then the intervals", func(t *testing.T) {
			assert.Equal(t, []string{"manual", "5m", "15m", "30m", "1h"}, s.Options)
		})
	})

	t.Run("when the config names an interval the panel does not offer", func(t *testing.T) {
		s := row(t, &config.Config{UsageRefresh: "45m"})

		t.Run("it should keep it in the cycle rather than drop it", func(t *testing.T) {
			assert.Contains(t, s.Options, "45m")
		})
	})

	t.Run("when the panel picks an interval", func(t *testing.T) {
		key, value := settingValue(ui.Setting{Key: config.KeyUsageRefresh, Value: "15m"})

		t.Run("it should save it as written", func(t *testing.T) {
			assert.Equal(t, []any{config.KeyUsageRefresh, "15m"}, []any{key, value})
		})
	})
}
