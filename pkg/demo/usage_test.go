package demo

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
)

func TestUsage(t *testing.T) {
	now := time.Date(2026, 10, 9, 15, 0, 0, 0, time.UTC)
	corpus := newCorpus(t)
	scenario := Scenario{
		MCP:          []string{"tracker"},
		Sessions:     []Placement{{Ref: "tidepool/6f1d", State: "working"}},
		Usage:        &Usage{Session: 42, Weekly: 61, Models: map[string]float64{"Opus": 18}},
		UsageChanges: []UsageChange{{At: Duration(10 * time.Second), Usage: Usage{Session: 87, Weekly: 64, Models: map[string]float64{"Opus": 18}}}},
	}

	t.Run("when a scenario sets the account's usage", func(t *testing.T) {
		got := stage(t, corpus, scenario, now)
		require.NoError(t, got.err)
		u, ok := claude.ReadUtilization(got.home)
		require.True(t, ok)
		var titles []string
		for _, g := range city.Gauges(u) {
			titles = append(titles, g.Title)
		}

		t.Run("it should float a boat per limit", func(t *testing.T) {
			assert.ElementsMatch(t, []string{"this session", "this week, every model", "this week, Opus"}, titles)
		})

		t.Run("it should read the session at its percentage", func(t *testing.T) {
			assert.Equal(t, 42.0, u.Limits[0].Percent)
		})
	})

	t.Run("when the usage changes partway through a clip", func(t *testing.T) {
		home := t.TempDir()
		d, err := NewDirector(corpus, scenario, home, now, func() (int, error) { return 9001, nil })
		require.NoError(t, err)
		for _, e := range d.Events() {
			require.NoError(t, d.Apply(e, now.Add(e.At)))
		}
		u, ok := claude.ReadUtilization(home)
		require.True(t, ok)

		t.Run("it should move the session boat", func(t *testing.T) {
			assert.Equal(t, 87.0, u.Limits[0].Percent)
		})

		t.Run("it should flag a limit running high, as Claude does", func(t *testing.T) {
			assert.Equal(t, "warning", u.Limits[0].Severity)
		})

		t.Run("it should keep the MCP servers the scenario configured", func(t *testing.T) {
			config, err := claude.ReadMCPConfig(home + "/.claude.json")
			require.NoError(t, err)
			assert.Len(t, config.Global, len(scenario.MCP))
		})
	})
}
