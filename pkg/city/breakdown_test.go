package city_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/stats"
)

func hourAgo(h int) int64 { return now.Add(-time.Duration(h)*time.Hour).Unix() / 3600 }

func spender(id, cwd, model string, hours map[int]int64) state.Session {
	s := session(id, cwd, state.Working)
	s.Model = model
	s.Hourly = map[int64]claude.Usage{}
	var total int64
	for h, out := range hours {
		s.Hourly[hourAgo(h)] = claude.Usage{Output: out, CacheCreate: out, Messages: 1}
		total += out
	}
	s.Usage = claude.Usage{Output: total}
	return s
}

// fakeLedger answers every window with the same store figures, or
// refuses with a reason.
type fakeLedger struct {
	window   stats.Window
	sessions map[string]stats.Cost
	reason   string
	asked    []int
}

func (l *fakeLedger) SessionCosts(at time.Time) (map[string]stats.Cost, error) {
	if l.reason != "" {
		return nil, errors.New(l.reason)
	}
	return l.sessions, nil
}

func (l *fakeLedger) Window(at time.Time, hours int, root func(string) string) (stats.Window, error) {
	l.asked = append(l.asked, hours)
	if l.reason != "" {
		return stats.Window{}, errors.New(l.reason)
	}
	return l.window, nil
}

func (l *fakeLedger) Reason() string { return l.reason }

func (l *fakeLedger) ReadFresh() {}

func cost(usd float64) stats.Cost { return stats.Cost{Low: usd, High: usd} }

var stored = stats.Window{
	Counters: stats.Counters{Input: 10, Write1h: 20, Read: 300, Output: 40, Messages: 4},
	Cost:     cost(2.5),
	ByModel: []stats.Slice{
		{Key: "claude-opus-5-5", Counters: stats.Counters{Output: 30, Messages: 3}, Cost: cost(2)},
		{Key: "claude-haiku-4-5", Counters: stats.Counters{Output: 10, Messages: 1}, Cost: cost(0.5)},
	},
	ByProject: []stats.Slice{{Key: cinders, Cost: cost(2.5)}},
	BySession: []stats.Slice{{Key: "a", Cost: cost(2.5)}},
	Hourly:    []stats.Cost{cost(1), cost(1.5)},
}

func TestBreakdown(t *testing.T) {
	sessions := []state.Session{
		spender("a", cinders, "claude-opus-5", map[int]int64{0: 100, 3: 200, 30: 400}),
		spender("b", cinders, "claude-sonnet-5", map[int]int64{1: 50}),
		spender("c", botropolis, "claude-opus-5", map[int]int64{100: 1000}),
	}

	t.Run("when the store can be read", func(t *testing.T) {
		ledger := &fakeLedger{window: stored}
		c := city.BuildWithLedger(snapshot(sessions...), city.NewLayout(), ledger)
		b := c.Breakdown(city.LastDay)

		t.Run("it should be priced", func(t *testing.T) {
			assert.True(t, b.Priced)
		})

		t.Run("it should ask the store for the window's hours", func(t *testing.T) {
			assert.Contains(t, ledger.asked, 24)
		})

		t.Run("it should carry the store's counters", func(t *testing.T) {
			assert.Equal(t, stored.Counters, b.Counters)
		})

		t.Run("it should carry the store's cost", func(t *testing.T) {
			assert.Equal(t, stored.Cost, b.Cost)
		})

		t.Run("it should split by each message's own model", func(t *testing.T) {
			assert.Equal(t, city.Share{Key: "claude-opus-5-5", Counters: stats.Counters{Output: 30, Messages: 3}, Cost: cost(2)}, b.ByModel[0])
		})

		t.Run("it should name each project by its directory's base name", func(t *testing.T) {
			assert.Equal(t, "cinders", b.ByProject[0].Key)
		})

		t.Run("it should give the cost of each hour as its series", func(t *testing.T) {
			assert.Equal(t, []float64{1, 1.5}, c.Series(city.LastDay))
		})
	})

	t.Run("when the store cannot be read", func(t *testing.T) {
		c := city.BuildWithLedger(snapshot(sessions...), city.NewLayout(), &fakeLedger{reason: "the store is being upgraded"})

		t.Run("and the last day is broken down", func(t *testing.T) {
			b := c.Breakdown(city.LastDay)

			t.Run("it should not be priced", func(t *testing.T) {
				assert.False(t, b.Priced)
			})

			t.Run("it should say why", func(t *testing.T) {
				assert.Equal(t, "the store is being upgraded", b.Reason)
			})

			t.Run("it should count the day's counters from the sessions, writes of unknown lifetime", func(t *testing.T) {
				assert.Equal(t, stats.Counters{Output: 350, WriteUnsplit: 350, Messages: 3}, b.Counters)
			})

			t.Run("it should price nothing", func(t *testing.T) {
				assert.Equal(t, stats.Cost{Unpriced: 3}, b.Cost)
			})

			t.Run("it should give no split by model", func(t *testing.T) {
				assert.Empty(t, b.ByModel)
			})

			t.Run("it should split by project", func(t *testing.T) {
				require.Len(t, b.ByProject, 1)
				assert.Equal(t, "cinders", b.ByProject[0].Key)
			})

			t.Run("it should list the sessions, most messages first", func(t *testing.T) {
				assert.Equal(t, []string{"a", "b"}, []string{b.BySession[0].Key, b.BySession[1].Key})
			})
		})

		t.Run("and the last hour is broken down, it should count only that hour", func(t *testing.T) {
			assert.Equal(t, int64(100), c.Breakdown(city.LastHour).Counters.Output)
		})

		t.Run("and the last week is broken down, it should reach the older session too", func(t *testing.T) {
			assert.Len(t, c.Breakdown(city.LastWeek).ByProject, 2)
		})

		t.Run("and a series is asked for, it should give messages an hour, oldest first", func(t *testing.T) {
			series := c.Series(city.LastDay)
			require.Len(t, series, 24)
			assert.Equal(t, []float64{1, 1}, []float64{series[23], series[20]})
		})
	})

	t.Run("when there is no ledger, it should say statistics are off", func(t *testing.T) {
		b := city.Build(snapshot(sessions...), city.NewLayout()).Breakdown(city.LastDay)
		assert.Equal(t, "statistics are off", b.Reason)
	})

	t.Run("when a session's series is asked for, it should cover the window", func(t *testing.T) {
		c := city.Build(snapshot(sessions...), city.NewLayout())
		assert.Len(t, c.Buildings()[0].Series(city.LastDay, now), 24)
	})
}
