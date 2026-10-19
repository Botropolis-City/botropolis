package stats_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/stats"
)

var someday = time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

func TestRateLookup(t *testing.T) {
	cases := []struct {
		name, model string
		want        string
	}{
		{"when the id has a date suffix, it should match its model's row", "claude-haiku-4-5-20251001", "claude-haiku-4-5"},
		{"when two rows share a prefix, it should match the longer", "claude-opus-5-5", "claude-opus-5-5"},
		{"when two rows share a prefix, it should match the shorter only exactly", "claude-opus-5", "claude-opus-5"},
		{"when the model is unknown, it should report no rate", "claude-opus-9-9", ""},
		{"when the id only starts like a known one, it should report no rate", "claude-opus-5-6", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rate, ok := stats.Prices.Lookup(c.model, someday)
			got := ""
			if ok {
				got = rate.Model
			}
			assert.Equal(t, c.want, got)
		})
	}

	t.Run("when a row has a later effective date", func(t *testing.T) {
		later := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
		rates := stats.Rates{
			{Model: "claude-x", Input: 2},
			{Model: "claude-x", From: later, Input: 3},
		}

		t.Run("it should price a message before that date by the earlier row", func(t *testing.T) {
			rate, _ := rates.Lookup("claude-x", later.Add(-time.Hour))
			assert.Equal(t, 2.0, rate.Input)
		})

		t.Run("it should price a message from that date by the later row", func(t *testing.T) {
			rate, _ := rates.Lookup("claude-x", later)
			assert.Equal(t, 3.0, rate.Input)
		})
	})

	t.Run("every tiered row should switch at the threshold the queries group by", func(t *testing.T) {
		ok := true
		for _, r := range stats.Prices {
			ok = ok && (r.LongAbove == 0 || r.LongAbove == stats.LongPrompt)
		}
		assert.True(t, ok)
	})
}

const million = 1_000_000

func priced(model string, c stats.Counters) stats.Priced {
	return stats.Priced{Model: model, At: someday, Counters: c}
}

func TestPrice(t *testing.T) {
	cases := []struct {
		name string
		in   stats.Priced
		low  float64
	}{
		{"when standard, it should price input at the input rate", priced("claude-opus-5-5", stats.Counters{Input: million}), 4},
		{"when standard, it should price 5 m writes at their rate", priced("claude-opus-5-5", stats.Counters{Write5m: million}), 5},
		{"when standard, it should price 1 h writes at their rate", priced("claude-opus-5-5", stats.Counters{Write1h: million}), 8},
		{"when standard, it should price reads at the row's own read rate", priced("claude-opus-5-5", stats.Counters{Read: million}), 0.20},
		{"when standard, it should price output at the output rate", priced("claude-opus-5-5", stats.Counters{Output: million}), 20},
		{"when the model reads at 0.025x, it should use that rate", priced("claude-fable-5-1", stats.Counters{Read: million}), 0.25},
		{"when fast, it should use the fast input rate", stats.Priced{Model: "claude-opus-5-5", Speed: "fast", At: someday, Counters: stats.Counters{Input: million}}, 8},
		{"when fast, it should use the fast output rate", stats.Priced{Model: "claude-opus-5-5", Speed: "fast", At: someday, Counters: stats.Counters{Output: million}}, 40},
		{"when fast, it should put the 1 h write multiplier on the fast input rate", stats.Priced{Model: "claude-opus-5-5", Speed: "fast", At: someday, Counters: stats.Counters{Write1h: million}}, 16},
		{"when fast, it should put the read multiplier on the fast input rate", stats.Priced{Model: "claude-opus-5-5", Speed: "fast", At: someday, Counters: stats.Counters{Read: million}}, 0.40},
		{"when in the US, it should be 1.1 times every counter", stats.Priced{Model: "claude-opus-5-5", Geo: "us", At: someday, Counters: stats.Counters{Input: million, Output: million}}, 26.4},
		{"when Haiku 5.5's prompt is over the threshold, it should use the higher tier", stats.Priced{Model: "claude-haiku-5-5", Long: true, At: someday, Counters: stats.Counters{Input: million}}, 0.50},
		{"when Haiku 5.5's prompt is not over it, it should use the lower tier", priced("claude-haiku-5-5", stats.Counters{Input: million}), 0.10},
		{"when the split is unknown, its low end should price the writes at 5 m", priced("claude-opus-5-5", stats.Counters{WriteUnsplit: million}), 5},
		{"when Sonnet 5.5 read from the cache before 7 October 2026, it should price the reads at the old rate",
			stats.Priced{Model: "claude-sonnet-5-5", At: time.Date(2026, 10, 6, 23, 0, 0, 0, time.UTC), Counters: stats.Counters{Read: million}}, 0.20},
		{"when Sonnet 5.5 read from the cache from 7 October 2026, it should price the reads at the cut rate",
			stats.Priced{Model: "claude-sonnet-5-5", At: time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC), Counters: stats.Counters{Read: million}}, 0.10},
		{"when it made web searches, it should add their per-use price", priced("claude-opus-5-5", stats.Counters{WebSearches: 1000}), 10},
		{"when it made web fetches, it should add nothing for them", priced("claude-opus-5-5", stats.Counters{WebFetches: 1000}), 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.InDelta(t, c.low, stats.Prices.Price(c.in).Low, 1e-9)
		})
	}

	t.Run("when the split is unknown, its high end should price the writes at 1 h", func(t *testing.T) {
		assert.InDelta(t, 8, stats.Prices.Price(priced("claude-opus-5-5", stats.Counters{WriteUnsplit: million})).High, 1e-9)
	})

	t.Run("when the split is known, its low and high ends should be equal", func(t *testing.T) {
		c := stats.Prices.Price(priced("claude-opus-5-5", stats.Counters{Input: 3, Write5m: 5, Write1h: 7, Read: 11, Output: 13}))
		assert.Equal(t, c.Low, c.High)
	})

	t.Run("when the model has no rate", func(t *testing.T) {
		c := stats.Prices.Price(priced("claude-opus-9-9", stats.Counters{Input: million, Messages: 3}))

		t.Run("it should count its messages as unpriced", func(t *testing.T) {
			assert.Equal(t, 3, c.Unpriced)
		})

		t.Run("it should add no dollars", func(t *testing.T) {
			assert.Equal(t, 0.0, c.High)
		})
	})

	t.Run("when the service tier is not standard", func(t *testing.T) {
		c := stats.Prices.Price(stats.Priced{Model: "claude-opus-5-5", Tier: "priority", At: someday, Counters: stats.Counters{Input: million, Messages: 2}})

		t.Run("it should count its messages as unpriced", func(t *testing.T) {
			assert.Equal(t, 2, c.Unpriced)
		})
	})

	t.Run("when the service tier is unrecorded, it should price it as standard", func(t *testing.T) {
		assert.InDelta(t, 4, stats.Prices.Price(priced("claude-opus-5-5", stats.Counters{Input: million})).Low, 1e-9)
	})

	t.Run("when two costs are added, it should add each end and the unpriced count", func(t *testing.T) {
		sum := stats.Cost{Low: 1, High: 2, Unpriced: 1}.Add(stats.Cost{Low: 3, High: 5, Unpriced: 2})
		assert.Equal(t, stats.Cost{Low: 4, High: 7, Unpriced: 3}, sum)
	})
}
