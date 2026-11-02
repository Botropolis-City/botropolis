package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Item 333: quiet_days is how long a loose session rests before the
// quiet sweep packs it up, and follows shuttered_days unless set.
func TestQuietDaysKey(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"when quiet_days is not set", nil, 7},
		{"when quiet_days is not set and shuttered_days is", []string{"--set", "shuttered_days=3"}, 3},
		{"when quiet_days is set", []string{"--set", "quiet_days=14"}, 14},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			cfg := load(t, c.args...)

			t.Run("it should be the days the sweep waits", func(t *testing.T) {
				assert.Equal(t, c.want, cfg.QuietDays)
			})
		})
	}

	t.Run("when quiet_days is set below zero", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		_, err := loadErr(t, "--set", "quiet_days=-1")

		t.Run("it should fail, saying what it takes", func(t *testing.T) {
			assert.ErrorContains(t, err, "quiet_days (-1) must be at least 1, or 0 to follow shuttered_days")
		})
	})
}
