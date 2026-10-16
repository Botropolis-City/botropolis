package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 87: the groups' lines are on by default, and a row hides them.
func TestGroupLinesSetting(t *testing.T) {
	row := func(t *testing.T, cfg *config.Config) ui.Setting {
		t.Helper()
		for _, s := range settingsFor(cfg).Items {
			if s.Key == config.KeyGroupLines {
				return s
			}
		}
		require.FailNow(t, "no group lines row")
		return ui.Setting{}
	}

	t.Run("when the config is the default", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		cfg, err := config.New(config.NewViper())
		require.NoError(t, err)
		s := row(t, cfg)

		t.Run("it should show the lines on", func(t *testing.T) {
			assert.Equal(t, "on", s.Value)
		})

		t.Run("it should offer on and off", func(t *testing.T) {
			assert.ElementsMatch(t, []string{"on", "off"}, s.Options)
		})
	})

	t.Run("when the panel turns them off", func(t *testing.T) {
		key, value := settingValue(ui.Setting{Key: config.KeyGroupLines, Value: "off"})

		t.Run("it should save false", func(t *testing.T) {
			assert.Equal(t, []any{config.KeyGroupLines, false}, []any{key, value})
		})
	})
}

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

// Item 86: how long a stopped session rests before it is shuttered, and
// before it is overgrown, are rows in the settings panel.
func TestLadderSettings(t *testing.T) {
	cfg := &config.Config{ShutteredDays: 7, OvergrownDays: 30}
	rows := map[string]ui.Setting{}
	for _, s := range settingsFor(cfg).Items {
		rows[s.Key] = s
	}

	t.Run("when the panel is built", func(t *testing.T) {
		t.Run("it should have no parked days row", func(t *testing.T) {
			assert.NotContains(t, rows, "parked_days")
		})

		for _, c := range []struct{ key, label, value string }{
			{config.KeyShutteredDays, "shuttered days", "7"},
			{config.KeyOvergrownDays, "overgrown days", "30"},
		} {
			t.Run("it should label the "+c.key+" row", func(t *testing.T) {
				assert.Equal(t, c.label, rows[c.key].Label)
			})

			t.Run("it should show "+c.key+" from the config", func(t *testing.T) {
				assert.Equal(t, c.value, rows[c.key].Value)
			})
		}
	})

	t.Run("when the panel picks a number of days", func(t *testing.T) {
		for _, k := range []string{config.KeyShutteredDays, config.KeyOvergrownDays} {
			key, value := settingValue(ui.Setting{Key: k, Value: "14"})

			t.Run("it should save "+k+" as a number", func(t *testing.T) {
				assert.Equal(t, []any{k, 14}, []any{key, value})
			})
		}
	})
}

// Item 84: the global shortcut is a row that reads off until one is set,
// and off saves as empty.
func TestGlobalShortcutSetting(t *testing.T) {
	row := func(t *testing.T, cfg *config.Config) ui.Setting {
		t.Helper()
		for _, s := range settingsFor(cfg).Items {
			if s.Key == config.KeyGlobalShortcut {
				return s
			}
		}
		require.FailNow(t, "no global shortcut row")
		return ui.Setting{}
	}

	t.Run("when nothing is set", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		cfg, err := config.New(config.NewViper())
		require.NoError(t, err)
		s := row(t, cfg)

		t.Run("it should read global shortcut, off", func(t *testing.T) {
			assert.Equal(t, [2]string{"global shortcut", "off"}, [2]string{s.Label, s.Value})
		})
	})

	t.Run("when the panel clears it", func(t *testing.T) {
		key, value := settingValue(ui.Setting{Key: config.KeyGlobalShortcut, Value: "off"})

		t.Run("it should save it empty", func(t *testing.T) {
			assert.Equal(t, []any{config.KeyGlobalShortcut, ""}, []any{key, value})
		})
	})
}

// Item 89: how much the city moves is a row in the settings panel, from
// still to lively, and it replaces the reduced motion row.
func TestLivelinessSetting(t *testing.T) {
	load := func(t *testing.T) *config.Config {
		t.Helper()
		cfg, err := config.New(config.NewViper())
		require.NoError(t, err)
		return cfg
	}
	row := func(t *testing.T, cfg *config.Config) ui.Setting {
		t.Helper()
		for _, s := range settingsFor(cfg).Items {
			if s.Key == config.KeyLiveliness {
				return s
			}
		}
		require.FailNow(t, "no liveliness row")
		return ui.Setting{}
	}

	t.Run("when the config is the default", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		s := row(t, load(t))

		t.Run("it should show a liveliness row reading steady", func(t *testing.T) {
			assert.Equal(t, "steady", s.Value)
		})

		t.Run("it should offer the levels from still to lively", func(t *testing.T) {
			assert.Equal(t, []string{"still", "calm", "steady", "lively"}, s.Options)
		})
	})

	t.Run("when the liveliness row is changed to calm", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		require.NoError(t, saveSetting("", ui.Setting{Key: config.KeyLiveliness, Value: "calm"}))

		t.Run("it should persist liveliness = calm", func(t *testing.T) {
			assert.Equal(t, "calm", load(t).Liveliness)
		})
	})

	t.Run("when reduced motion was saved on", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		require.NoError(t, config.Save("", config.KeyReducedMotion, true))

		t.Run("and the liveliness row is changed to calm", func(t *testing.T) {
			require.NoError(t, saveSetting("", ui.Setting{Key: config.KeyLiveliness, Value: "calm"}))

			t.Run("it should load as calm, because the row is the newer choice", func(t *testing.T) {
				assert.Equal(t, "calm", load(t).Liveliness)
			})
		})
	})
}
