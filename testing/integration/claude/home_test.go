package claude_test

import (
	"path/filepath"
	"testing"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/testing/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadStatsFromFixture(t *testing.T) {
	t.Run("when reading the sample fixture's stats cache", func(t *testing.T) {
		stats, err := claude.ReadStats(filepath.Join(helpers.FixtureHome(t, "sample"), ".claude", "stats-cache.json"))
		require.NoError(t, err)

		t.Run("it should know when it was computed", func(t *testing.T) {
			assert.NotEmpty(t, stats.LastComputed)
		})

		t.Run("it should have daily activity", func(t *testing.T) {
			assert.NotEmpty(t, stats.DailyActivity)
		})

		t.Run("it should have per-model usage", func(t *testing.T) {
			assert.NotEmpty(t, stats.ModelUsage)
		})

		t.Run("it should count sessions", func(t *testing.T) {
			assert.Positive(t, stats.TotalSessions)
		})

		t.Run("it should have an hour with activity", func(t *testing.T) {
			total := 0
			for _, n := range stats.HourCounts {
				total += n
			}
			assert.Positive(t, total)
		})
	})
}

func TestReadMCPConfigFromFixture(t *testing.T) {
	t.Run("when reading the sample fixture's claude.json", func(t *testing.T) {
		config, err := claude.ReadMCPConfig(filepath.Join(helpers.FixtureHome(t, "sample"), ".claude.json"))
		require.NoError(t, err)

		t.Run("it should list global servers", func(t *testing.T) {
			assert.NotEmpty(t, config.Global)
		})

		t.Run("it should give every server a type", func(t *testing.T) {
			for _, s := range config.Global {
				assert.NotEmpty(t, s.Type, s.Name)
			}
		})

		t.Run("it should list at least as many servers for a project as globally", func(t *testing.T) {
			for cwd := range config.Projects {
				assert.GreaterOrEqual(t, len(config.ForProject(cwd)), len(config.Global), cwd)
			}
		})
	})
}
