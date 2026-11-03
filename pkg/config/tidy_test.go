package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/config"
)

// Item 333: tidy_days is how long away the map must have been for the
// tidy-up walk to open by itself, and it must be longer than the gap
// that opens the journal.
func TestTidyDaysKey(t *testing.T) {
	t.Run("when tidy_days is not set", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		cfg := load(t)

		t.Run("it should be a week", func(t *testing.T) {
			assert.Equal(t, config.DefaultTidyDays, cfg.TidyDays)
		})
	})

	t.Run("when tidy_days is set", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		cfg := load(t, "--set", "tidy_days=3")

		t.Run("it should be the days set", func(t *testing.T) {
			assert.Equal(t, 3, cfg.TidyDays)
		})
	})

	for _, c := range []struct {
		name string
		args []string
		want string
	}{
		{"when tidy_days is no longer than journal_hours", []string{"--set", "tidy_days=1", "--set", "journal_hours=24"},
			"tidy_days (1, 24 hours) must be longer than journal_hours (24)"},
		{"when tidy_days is under one", []string{"--set", "tidy_days=0"}, "tidy_days (0) must be at least 1"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			_, err := loadErr(t, c.args...)

			t.Run("it should fail, naming both", func(t *testing.T) {
				assert.ErrorContains(t, err, c.want)
			})
		})
	}
}
