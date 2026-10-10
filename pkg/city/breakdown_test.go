package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

func hourAgo(h int) int64 { return now.Add(-time.Duration(h)*time.Hour).Unix() / 3600 }

func spender(id, cwd, model string, hours map[int]int64) state.Session {
	s := session(id, cwd, state.Working)
	s.Model = model
	s.Hourly = map[int64]claude.Usage{}
	var total int64
	for h, out := range hours {
		s.Hourly[hourAgo(h)] = claude.Usage{Output: out}
		total += out
	}
	s.Usage = claude.Usage{Output: total}
	s.CostUSD = float64(total) / 1000
	s.CostKnown = true
	return s
}

func TestBreakdown(t *testing.T) {
	c := build(t, city.NewLayout(),
		spender("a", cinders, "claude-opus-5", map[int]int64{0: 100, 3: 200, 30: 400}),
		spender("b", cinders, "claude-sonnet-5", map[int]int64{1: 50}),
		spender("c", botropolis, "claude-opus-5", map[int]int64{100: 1000}),
	)

	t.Run("when the last hour is broken down", func(t *testing.T) {
		b := c.Breakdown(city.LastHour)

		t.Run("it should count only that hour's tokens", func(t *testing.T) {
			assert.Equal(t, int64(100), b.Tokens)
		})

		t.Run("it should list the sessions that spent, most first", func(t *testing.T) {
			require.Len(t, b.BySession, 1)
			assert.Equal(t, "a", b.BySession[0].Key)
		})
	})

	t.Run("when the last day is broken down", func(t *testing.T) {
		b := c.Breakdown(city.LastDay)

		t.Run("it should sum the day's tokens", func(t *testing.T) {
			assert.Equal(t, int64(350), b.Tokens)
		})

		t.Run("it should split by model, most first", func(t *testing.T) {
			require.Len(t, b.ByModel, 2)
			assert.Equal(t, city.Share{Key: "claude-opus-5", Tokens: 300, CostUSD: 0.3, CostKnown: true}, b.ByModel[0])
			assert.Equal(t, city.Share{Key: "claude-sonnet-5", Tokens: 50, CostUSD: 0.05, CostKnown: true}, b.ByModel[1])
		})

		t.Run("it should split by project", func(t *testing.T) {
			require.Len(t, b.ByProject, 1)
			assert.Equal(t, "cinders", b.ByProject[0].Key)
		})

		t.Run("it should pro-rate each session's cost by the window's share of its tokens", func(t *testing.T) {
			assert.InDelta(t, 0.3+0.05, b.CostUSD, 1e-9)
		})
	})

	t.Run("when the last week is broken down", func(t *testing.T) {
		b := c.Breakdown(city.LastWeek)

		t.Run("it should reach the older session too", func(t *testing.T) {
			assert.Equal(t, int64(1750), b.Tokens)
			assert.Len(t, b.ByProject, 2)
		})
	})

	t.Run("when no session carries a cost", func(t *testing.T) {
		unpriced := spender("a", cinders, "claude-opus-5", map[int]int64{0: 100})
		unpriced.CostUSD, unpriced.CostKnown = 0, false
		b := build(t, city.NewLayout(), unpriced).Breakdown(city.LastDay)

		t.Run("it should not know the cost", func(t *testing.T) {
			assert.False(t, b.CostKnown)
		})

		t.Run("it should not know any share's cost", func(t *testing.T) {
			require.Len(t, b.ByModel, 1)
			assert.False(t, b.ByModel[0].CostKnown)
		})
	})

	t.Run("when a series is asked for", func(t *testing.T) {
		t.Run("it should give the city's tokens per hour over the day, oldest first", func(t *testing.T) {
			series := c.Series(city.LastDay)
			require.Len(t, series, 24)
			assert.Equal(t, 100.0, series[23])
			assert.Equal(t, 200.0, series[20])
		})

		t.Run("it should give a session's tokens per hour over its life", func(t *testing.T) {
			series := c.Buildings()[0].Series(city.LastDay, now)
			assert.Len(t, series, 24)
		})
	})
}
