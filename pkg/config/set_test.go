package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
)

// parsed runs args through a command carrying the config flags and
// returns the parse error, the loaded config, and the deprecation notes.
func parsed(t *testing.T, args ...string) (*config.Config, []string, error) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var notes []string
	cmd := &cobra.Command{Use: "test", SilenceErrors: true, SilenceUsage: true, RunE: func(cmd *cobra.Command, _ []string) error {
		notes = config.DeprecationNotes(cmd.Flags())
		return nil
	}}
	v := config.NewViper()
	config.BindFlags(v, cmd.PersistentFlags())
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		return nil, nil, err
	}
	cfg, err := config.New(v)
	return cfg, notes, err
}

func TestSetFlag(t *testing.T) {
	for _, c := range []struct {
		scenario string
		set      string
		got      func(*config.Config) any
		want     any
	}{
		{"when --set names a string key", "liveliness=calm", func(c *config.Config) any { return c.Liveliness }, "calm"},
		{"when --set names an int key", "shuttered_days=3", func(c *config.Config) any { return c.ShutteredDays }, 3},
		{"when --set names a bool key", "stats=false", func(c *config.Config) any { return c.Stats }, false},
		{"when --set names a float key", "daily_budget_usd=2.5", func(c *config.Config) any { return c.DailyBudget }, 2.5},
		{"when --set names a key that never had a flag", "terminal=kitty", func(c *config.Config) any { return c.Terminal }, "kitty"},
	} {
		t.Run(c.scenario, func(t *testing.T) {
			cfg, _, err := parsed(t, "--set", c.set)
			require.NoError(t, err)

			t.Run("it should reach the loaded config", func(t *testing.T) {
				assert.Equal(t, c.want, c.got(cfg))
			})
		})
	}

	t.Run("when --set is repeated", func(t *testing.T) {
		cfg, _, err := parsed(t, "--set", "shuttered_days=3", "--set", "overgrown_days=40")
		require.NoError(t, err)

		t.Run("it should apply each", func(t *testing.T) {
			assert.Equal(t, [2]int{3, 40}, [2]int{cfg.ShutteredDays, cfg.OvergrownDays})
		})
	})

	t.Run("when the config file sets the same key", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "config.toml")
		require.NoError(t, os.WriteFile(file, []byte("liveliness = \"lively\"\n"), 0o600))
		cfg, _, err := parsed(t, "--config", file, "--set", "liveliness=calm")
		require.NoError(t, err)

		t.Run("it should take --set over the file", func(t *testing.T) {
			assert.Equal(t, "calm", cfg.Liveliness)
		})
	})

	t.Run("when --set names an unknown key", func(t *testing.T) {
		_, _, err := parsed(t, "--set", "colour=red")

		t.Run("it should fail naming the key and the manual page", func(t *testing.T) {
			assert.ErrorContains(t, err, `unknown key "colour": see botropolis-config(5)`)
		})
	})

	t.Run("when --set has no equals sign", func(t *testing.T) {
		_, _, err := parsed(t, "--set", "liveliness")

		t.Run("it should ask for key=value", func(t *testing.T) {
			assert.ErrorContains(t, err, `"liveliness" is not key=value`)
		})
	})
}

func TestDeprecatedKeyFlags(t *testing.T) {
	for _, c := range []struct{ key, value string }{
		{"shuttered_days", "3"},
		{"overgrown_days", "40"},
		{"render_scale", "2"},
		{"reduced_motion", "true"},
		{"liveliness", "calm"},
		{"district_type", "guess"},
		{"stats", "false"},
		{"daily_budget_usd", "5"},
		{"detail", "plain"},
		{"usage_refresh", "30m"},
		{"group_lines", "false"},
		{"labels", "false"},
		{"find_key", "mod+j"},
		{"find_copy_key", "mod+u"},
		{"find_show_key", "ctrl+o"},
		{"new_session_key", "shift+b"},
		{"explain_key", "x"},
		{"triage_key", "v"},
		{"journal_key", "shift+j"},
		{"journal_hours", "4"},
		{"stream_key", "o"},
		{"city_plan_key", "n"},
		{"labels_key", "y"},
	} {
		t.Run("when the old flag --"+c.key+" is passed", func(t *testing.T) {
			byFlag, notes, err := parsed(t, "--"+c.key+"="+c.value)
			require.NoError(t, err)
			bySet, _, err := parsed(t, "--set", c.key+"="+c.value)
			require.NoError(t, err)
			byDefault, _, err := parsed(t)
			require.NoError(t, err)

			t.Run("it should set the key as --set would", func(t *testing.T) {
				assert.Equal(t, bySet, byFlag)
			})

			t.Run("it should change the config from its default", func(t *testing.T) {
				assert.NotEqual(t, byDefault, byFlag)
			})

			t.Run("it should give one note naming --set", func(t *testing.T) {
				assert.Equal(t, []string{"--" + c.key + " is deprecated and goes at 1.0: use --set " + c.key + "=" + c.value}, notes)
			})

			t.Run("and help is printed, it should not list it", func(t *testing.T) {
				cmd := &cobra.Command{Use: "test"}
				config.BindFlags(config.NewViper(), cmd.Flags())
				assert.True(t, cmd.Flags().Lookup(c.key).Hidden)
			})
		})
	}

	t.Run("when only --set is passed", func(t *testing.T) {
		_, notes, err := parsed(t, "--set", "liveliness=calm")
		require.NoError(t, err)

		t.Run("it should give no note", func(t *testing.T) {
			assert.Empty(t, notes)
		})
	})
}
