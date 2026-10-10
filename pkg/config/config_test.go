package config_test

import (
	"github.com/spf13/pflag"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/config"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func load(t *testing.T, args ...string) *config.Config {
	t.Helper()
	cmd := &cobra.Command{Use: "test", RunE: func(*cobra.Command, []string) error { return nil }}
	v := config.NewViper()
	config.BindFlags(v, cmd.PersistentFlags())
	cmd.SetArgs(args)
	require.NoError(t, cmd.Execute())
	cfg, err := config.New(v)
	require.NoError(t, err)
	return cfg
}

func TestConfig(t *testing.T) {
	t.Run("when nothing is set", func(t *testing.T) {
		t.Setenv("HOME", "/home/someone")
		t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		t.Setenv("BOTROPOLIS_HOME", "")
		t.Setenv("BOTROPOLIS_SOCKET", "")
		t.Setenv("BOTROPOLIS_TERMINAL", "")
		cfg := load(t)

		t.Run("it should default home to $HOME", func(t *testing.T) {
			assert.Equal(t, "/home/someone", cfg.Home)
		})

		t.Run("it should default the socket to the runtime dir", func(t *testing.T) {
			assert.Equal(t, "/run/user/1000/botropolis/botropolis.sock", cfg.Socket)
		})

		t.Run("it should default the hook command to the bare binary name", func(t *testing.T) {
			assert.Equal(t, "botropolis-hook", cfg.HookCommand)
		})

		t.Run("it should leave the terminal empty", func(t *testing.T) {
			assert.Empty(t, cfg.Terminal)
		})

		t.Run("it should default to a week of parked sessions", func(t *testing.T) {
			assert.Equal(t, 7, cfg.ParkedDays)
		})

		t.Run("it should default the codex home under home", func(t *testing.T) {
			assert.Equal(t, "/home/someone/.codex", cfg.CodexHome)
		})
	})

	t.Run("when parked days is given as a flag", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		cfg := load(t, "--parked_days", "30")

		t.Run("it should take it", func(t *testing.T) {
			assert.Equal(t, 30, cfg.ParkedDays)
		})
	})

	t.Run("when flags are given", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		cfg := load(t, "--home", "/tmp/h", "--socket", "/tmp/s.sock")

		t.Run("it should take home from the flag", func(t *testing.T) {
			assert.Equal(t, "/tmp/h", cfg.Home)
		})

		t.Run("it should take the socket from the flag", func(t *testing.T) {
			assert.Equal(t, "/tmp/s.sock", cfg.Socket)
		})

		t.Run("it should know home was set explicitly", func(t *testing.T) {
			assert.True(t, cfg.HomeSet)
		})

		t.Run("it should know the socket was set explicitly", func(t *testing.T) {
			assert.True(t, cfg.SocketSet)
		})
	})

	t.Run("when environment variables are set", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		t.Setenv("BOTROPOLIS_HOME", "/env/home")
		t.Setenv("BOTROPOLIS_TERMINAL", "wezterm start --")
		cfg := load(t)

		t.Run("it should take home from BOTROPOLIS_HOME", func(t *testing.T) {
			assert.Equal(t, "/env/home", cfg.Home)
		})

		t.Run("it should take the terminal from BOTROPOLIS_TERMINAL", func(t *testing.T) {
			assert.Equal(t, "wezterm start --", cfg.Terminal)
		})

		t.Run("and a flag is also given", func(t *testing.T) {
			cfg := load(t, "--home", "/flag/home")

			t.Run("it should let the flag win", func(t *testing.T) {
				assert.Equal(t, "/flag/home", cfg.Home)
			})
		})
	})

	t.Run("when a config file exists under XDG_CONFIG_HOME", func(t *testing.T) {
		configHome := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", configHome)
		t.Setenv("BOTROPOLIS_TERMINAL", "")
		require.NoError(t, os.MkdirAll(filepath.Join(configHome, "botropolis"), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(configHome, "botropolis", "config.toml"),
			[]byte("terminal = \"foot\"\nhook_command = \"/usr/bin/botropolis-hook\"\n"), 0o600))
		cfg := load(t)

		t.Run("it should read the terminal from the file", func(t *testing.T) {
			assert.Equal(t, "foot", cfg.Terminal)
		})

		t.Run("it should read the hook command from the file", func(t *testing.T) {
			assert.Equal(t, "/usr/bin/botropolis-hook", cfg.HookCommand)
		})

		t.Run("it should report the file it used", func(t *testing.T) {
			assert.Equal(t, filepath.Join(configHome, "botropolis", "config.toml"), cfg.File)
		})
	})

	t.Run("when the config file is malformed", func(t *testing.T) {
		configHome := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", configHome)
		require.NoError(t, os.MkdirAll(filepath.Join(configHome, "botropolis"), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(configHome, "botropolis", "config.toml"), []byte("terminal = [\n"), 0o600))
		v := config.NewViper()
		_, err := config.New(v)

		t.Run("it should return an error", func(t *testing.T) {
			assert.Error(t, err)
		})
	})
}

func TestProjectionKey(t *testing.T) {
	t.Run("when nothing sets the projection", func(t *testing.T) {
		v := config.NewViper()
		cfg, err := config.New(v)
		require.NoError(t, err)

		t.Run("it should draw the city isometric", func(t *testing.T) {
			assert.Equal(t, "iso", cfg.Projection)
		})
	})

	t.Run("when the flag asks for the top-down view", func(t *testing.T) {
		v := config.NewViper()
		flags := pflag.NewFlagSet("t", pflag.ContinueOnError)
		config.BindFlags(v, flags)
		require.NoError(t, flags.Parse([]string{"--projection", "top"}))
		cfg, err := config.New(v)
		require.NoError(t, err)

		t.Run("it should say top", func(t *testing.T) {
			assert.Equal(t, "top", cfg.Projection)
		})
	})
}

func TestRenderScaleKey(t *testing.T) {
	t.Run("when nothing sets the render scale", func(t *testing.T) {
		v := config.NewViper()
		cfg, err := config.New(v)
		require.NoError(t, err)

		t.Run("it should leave it to the display", func(t *testing.T) {
			assert.Zero(t, cfg.RenderScale)
		})
	})

	t.Run("when the flag asks for 2x", func(t *testing.T) {
		v := config.NewViper()
		flags := pflag.NewFlagSet("t", pflag.ContinueOnError)
		config.BindFlags(v, flags)
		require.NoError(t, flags.Parse([]string{"--render_scale", "2"}))
		cfg, err := config.New(v)
		require.NoError(t, err)

		t.Run("it should say 2", func(t *testing.T) {
			assert.Equal(t, 2.0, cfg.RenderScale)
		})
	})
}

func TestReducedMotionKey(t *testing.T) {
	t.Run("when nothing sets reduced motion", func(t *testing.T) {
		v := config.NewViper()
		cfg, err := config.New(v)
		require.NoError(t, err)

		t.Run("it should leave motion on", func(t *testing.T) {
			assert.False(t, cfg.ReducedMotion)
		})
	})

	t.Run("when the flag asks for reduced motion", func(t *testing.T) {
		v := config.NewViper()
		flags := pflag.NewFlagSet("t", pflag.ContinueOnError)
		config.BindFlags(v, flags)
		require.NoError(t, flags.Parse([]string{"--reduced_motion"}))
		cfg, err := config.New(v)
		require.NoError(t, err)

		t.Run("it should say so", func(t *testing.T) {
			assert.True(t, cfg.ReducedMotion)
		})
	})
}

func TestSave(t *testing.T) {
	t.Run("when a value is saved with no config file yet", func(t *testing.T) {
		configHome := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", configHome)
		path := filepath.Join(configHome, "botropolis", "config.toml")
		require.NoError(t, config.Save("", config.KeyProjection, "top"))
		cfg, err := config.New(config.NewViper())
		require.NoError(t, err)

		t.Run("it should create the file under the config home", func(t *testing.T) {
			assert.FileExists(t, path)
		})

		t.Run("it should read back the value", func(t *testing.T) {
			assert.Equal(t, "top", cfg.Projection)
		})
	})

	t.Run("when a value is saved beside existing keys", func(t *testing.T) {
		configHome := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", configHome)
		path := filepath.Join(configHome, "botropolis", "config.toml")
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, []byte("terminal = \"kitty\"\n"), 0o600))
		require.NoError(t, config.Save(path, config.KeyParkedDays, 14))
		cfg, err := config.New(config.NewViper())
		require.NoError(t, err)

		t.Run("it should keep the existing key", func(t *testing.T) {
			assert.Equal(t, "kitty", cfg.Terminal)
		})

		t.Run("it should hold the new value", func(t *testing.T) {
			assert.Equal(t, 14, cfg.ParkedDays)
		})

		t.Run("it should not write defaults for other keys", func(t *testing.T) {
			data, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.NotContains(t, string(data), "hook_command")
		})
	})
}

func TestDailyBudgetKey(t *testing.T) {
	t.Run("when nothing sets a budget", func(t *testing.T) {
		v := config.NewViper()
		cfg, err := config.New(v)
		require.NoError(t, err)

		t.Run("it should have none", func(t *testing.T) {
			assert.Zero(t, cfg.DailyBudget)
		})
	})

	t.Run("when the flag sets one", func(t *testing.T) {
		v := config.NewViper()
		flags := pflag.NewFlagSet("t", pflag.ContinueOnError)
		config.BindFlags(v, flags)
		require.NoError(t, flags.Parse([]string{"--daily_budget_usd", "250"}))
		cfg, err := config.New(v)
		require.NoError(t, err)

		t.Run("it should say 250", func(t *testing.T) {
			assert.Equal(t, 250.0, cfg.DailyBudget)
		})
	})
}

func TestDetailSetting(t *testing.T) {
	t.Run("when nothing sets it", func(t *testing.T) {
		t.Run("it should draw the whole city", func(t *testing.T) {
			v := config.NewViper()
			cfg, err := config.New(v)
			require.NoError(t, err)
			assert.Equal(t, "full", cfg.Detail)
		})
	})
}

// Item 79: refreshing usage is a network call, so the city makes it only
// when asked unless told how often it may.
func TestUsageRefreshKey(t *testing.T) {
	t.Run("when nothing sets it", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		cfg := load(t)

		t.Run("it should refresh only by hand", func(t *testing.T) {
			assert.Equal(t, "manual", cfg.UsageRefresh)
		})
	})

	t.Run("when the flag sets an interval", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		cfg := load(t, "--usage_refresh", "30m")

		t.Run("it should keep it", func(t *testing.T) {
			assert.Equal(t, "30m", cfg.UsageRefresh)
		})
	})

	t.Run("when the file names something that is not an interval", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", dir)
		require.NoError(t, config.Save("", config.KeyUsageRefresh, "often"))
		_, err := config.New(config.NewViper())

		t.Run("it should refuse to start rather than guess", func(t *testing.T) {
			assert.Error(t, err)
		})
	})
}

func TestParseUsageRefresh(t *testing.T) {
	for _, c := range []struct {
		value string
		every time.Duration
		ok    bool
	}{
		{"manual", 0, true},
		{"", 0, true},
		{"0", 0, true},
		{"5m", 5 * time.Minute, true},
		{"1h", time.Hour, true},
		{"often", 0, false},
		{"-5m", 0, false},
	} {
		t.Run("when it reads "+strconv.Quote(c.value), func(t *testing.T) {
			every, err := config.ParseUsageRefresh(c.value)

			t.Run("it should give the interval it names", func(t *testing.T) {
				assert.Equal(t, c.every, every)
			})

			t.Run("it should say whether that was an interval", func(t *testing.T) {
				assert.Equal(t, c.ok, err == nil)
			})
		})
	}
}
