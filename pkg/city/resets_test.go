package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
)

// Item 86, slice 5: a usage window that reset while you were away is
// told on the since strip, from the reset time the probe already reads.
func TestUsageResets(t *testing.T) {
	seen := now.Add(-64 * time.Hour)

	t.Run("when a reading has a reset time", func(t *testing.T) {
		at := now.Add(-time.Hour)
		gauges := city.Gauges(claude.Utilization{Limits: []claude.Limit{{Percent: 40, ResetsAt: at}}})
		require.Len(t, gauges, 1)

		t.Run("the gauge should keep it", func(t *testing.T) {
			assert.Equal(t, at, gauges[0].ResetsAt)
		})
	})

	for _, c := range []struct {
		name  string
		at    time.Time
		reset bool
	}{
		{"inside the gap", now.Add(-time.Hour), true},
		{"before the gap", seen.Add(-time.Hour), false},
		{"still to come", now.Add(time.Hour), false},
		{"not known", time.Time{}, false},
	} {
		t.Run("when a window's reset time is "+c.name, func(t *testing.T) {
			s := scene(t)
			s.SetGauges([]city.Gauge{{Title: "session", Percent: 40, ResetsAt: c.at}})

			t.Run("it should count it as reset only when it fell inside the gap", func(t *testing.T) {
				assert.Equal(t, c.reset, len(s.ResetsDuring(seen)) == 1)
			})
		})
	}
}
