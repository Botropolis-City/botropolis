package helpers

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/stats"
)

func TestStatsRanges(t *testing.T) {
	now := time.Date(2026, time.October, 19, 15, 0, 0, 0, time.Local)
	path := StatsRanges(t, t.TempDir(), now)
	store, err := stats.OpenReader(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	today := stats.DateOf(now)
	all := stats.Filter{Root: city.ProjectRoot}

	t.Run("when the store is read over the last 3 months", func(t *testing.T) {
		cov, err := store.Coverage(stats.Last3Months(today), all, time.Local)
		require.NoError(t, err)

		t.Run("it should hold data from the first day", func(t *testing.T) {
			assert.Equal(t, stats.DateOf(StatsRangesFirst), cov.Earliest)
		})

		t.Run("it should touch the gap", func(t *testing.T) {
			assert.True(t, cov.TouchesGap)
		})
	})

	t.Run("when its districts are listed", func(t *testing.T) {
		roots, err := store.Districts(city.ProjectRoot)
		require.NoError(t, err)

		t.Run("it should hold all three", func(t *testing.T) {
			assert.Len(t, roots, len(StatsRangeDistricts))
		})
	})

	t.Run("when its model mix is read", func(t *testing.T) {
		mix, err := store.ModelMix(stats.Last3Months(today), all, time.Local)
		require.NoError(t, err)

		t.Run("it should price every model", func(t *testing.T) {
			unpriced := 0
			for _, s := range mix {
				unpriced += s.Cost.Unpriced
			}
			assert.Zero(t, unpriced)
		})
	})
}

// TestWriteStatsRangesHome writes the store into the home named by
// STATS_RANGES_HOME, for filming the plant's long ranges with --home; it
// does nothing otherwise.
func TestWriteStatsRangesHome(t *testing.T) {
	home := os.Getenv("STATS_RANGES_HOME")
	if home == "" {
		t.Skip("STATS_RANGES_HOME is not set")
	}
	t.Log(StatsRanges(t, home, time.Now()))
}
