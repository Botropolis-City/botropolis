package claude_test

import (
	"path/filepath"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const statsCache = `{"version":4,"lastComputedDate":"2026-07-02",` +
	`"dailyActivity":[{"date":"2026-01-13","messageCount":1156,"sessionCount":8,"toolCallCount":348},` +
	`{"date":"2026-01-14","messageCount":20,"sessionCount":1,"toolCallCount":3}],` +
	`"dailyModelTokens":[{"date":"2026-01-13","tokensByModel":{"claude-sonnet-4-5-20250929":72739}}],` +
	`"modelUsage":{"claude-sonnet-4-5-20250929":{"inputTokens":417260,"outputTokens":758791,` +
	`"cacheReadInputTokens":1318498531,"cacheCreationInputTokens":77548853,"webSearchRequests":0,"costUSD":0,` +
	`"contextWindow":0,"maxOutputTokens":0}},` +
	`"totalSessions":513,"totalMessages":125894,` +
	`"longestSession":{"sessionId":"2eb7152c-d713-4b7f-9381-b886a9c0ffee","duration":1896768455,"messageCount":5031,` +
	`"timestamp":"2026-06-05T00:46:51.319Z"},` +
	`"firstSessionDate":"2026-01-13T17:49:58.240Z","hourCounts":{"0":4,"13":90,"23":7}}`

func TestReadStats(t *testing.T) {
	t.Run("when the file holds a well-formed cache", func(t *testing.T) {
		stats, err := claude.ReadStats(writeFile(t, t.TempDir(), "stats-cache.json", statsCache))
		require.NoError(t, err)

		t.Run("it should read the version", func(t *testing.T) {
			assert.Equal(t, 4, stats.Version)
		})

		t.Run("it should read the last computed date", func(t *testing.T) {
			assert.Equal(t, "2026-07-02", stats.LastComputed)
		})

		t.Run("it should read the daily activity", func(t *testing.T) {
			assert.Equal(t, []claude.DailyActivity{
				{Date: "2026-01-13", Messages: 1156, Sessions: 8, ToolCalls: 348},
				{Date: "2026-01-14", Messages: 20, Sessions: 1, ToolCalls: 3},
			}, stats.DailyActivity)
		})

		t.Run("it should read the daily tokens by model", func(t *testing.T) {
			assert.Equal(t, []claude.DailyModelTokens{
				{Date: "2026-01-13", TokensByModel: map[string]int64{"claude-sonnet-4-5-20250929": 72739}},
			}, stats.DailyModelTokens)
		})

		t.Run("it should read the per-model usage", func(t *testing.T) {
			assert.Equal(t, claude.ModelCost{Usage: claude.Usage{
				Input:       417260,
				Output:      758791,
				CacheRead:   1318498531,
				CacheCreate: 77548853,
			}}, stats.ModelUsage["claude-sonnet-4-5-20250929"])
		})

		t.Run("it should read the total sessions", func(t *testing.T) {
			assert.Equal(t, 513, stats.TotalSessions)
		})

		t.Run("it should read the total messages", func(t *testing.T) {
			assert.Equal(t, 125894, stats.TotalMessages)
		})

		t.Run("it should read the first session date", func(t *testing.T) {
			assert.Equal(t, "2026-01-13T17:49:58.240Z", stats.FirstSessionAt.UTC().Format("2006-01-02T15:04:05.000Z"))
		})

		t.Run("it should place the hour counts in their slots", func(t *testing.T) {
			assert.Equal(t, [24]int{0: 4, 13: 90, 23: 7}, stats.HourCounts)
		})
	})

	t.Run("when the file does not exist", func(t *testing.T) {
		stats, err := claude.ReadStats(filepath.Join(t.TempDir(), "stats-cache.json"))

		t.Run("it should not return an error", func(t *testing.T) {
			assert.NoError(t, err)
		})

		t.Run("it should return an empty cache", func(t *testing.T) {
			assert.Equal(t, claude.Stats{}, stats)
		})
	})

	t.Run("when the file is not valid JSON", func(t *testing.T) {
		_, err := claude.ReadStats(writeFile(t, t.TempDir(), "stats-cache.json", `{"version":`))

		t.Run("it should return an error", func(t *testing.T) {
			assert.Error(t, err)
		})
	})
}

func TestStatsRollups(t *testing.T) {
	stats := claude.Stats{
		LastComputed: "2026-07-02",
		DailyActivity: []claude.DailyActivity{
			{Date: "2026-07-02", Messages: 300, Sessions: 2},
			{Date: "2026-01-13", Messages: 1156, Sessions: 8},
			{Date: "2026-07-01", Messages: 100, Sessions: 1},
		},
		ModelUsage: map[string]claude.ModelCost{
			"claude-sonnet-4-6": {Usage: claude.Usage{Output: 6_000_000}},
			"claude-opus-4-8":   {Usage: claude.Usage{Output: 2_000_000}},
		},
		HourCounts: [24]int{0: 4, 9: 40, 14: 41, 23: 1},
	}

	t.Run("when the cache was computed on a date", func(t *testing.T) {
		t.Run("it should parse it", func(t *testing.T) {
			on, ok := stats.ComputedOn()
			require.True(t, ok)
			assert.Equal(t, time.Date(2026, time.July, 2, 0, 0, 0, 0, time.UTC), on)
		})
	})

	t.Run("when the hours have counts", func(t *testing.T) {
		t.Run("it should find the busiest hour", func(t *testing.T) {
			assert.Equal(t, 14, stats.BusiestHour())
		})
	})

	t.Run("when the days are out of order", func(t *testing.T) {
		t.Run("it should return the recent days oldest first", func(t *testing.T) {
			assert.Equal(t, []claude.DailyActivity{
				{Date: "2026-07-01", Messages: 100, Sessions: 1},
				{Date: "2026-07-02", Messages: 300, Sessions: 2},
			}, stats.RecentDays(2))
		})
	})

	t.Run("when two models have output", func(t *testing.T) {
		shares := stats.ModelShare()

		t.Run("it should rank them by share", func(t *testing.T) {
			require.Len(t, shares, 2)
			assert.Equal(t, "claude-sonnet-4-6", shares[0].Model)
			assert.InDelta(t, 0.75, shares[0].Share, 1e-9)
		})
	})

	t.Run("when nothing has been counted", func(t *testing.T) {
		t.Run("it should have no busiest hour", func(t *testing.T) {
			assert.Equal(t, -1, claude.Stats{}.BusiestHour())
		})

		t.Run("it should have no computed date", func(t *testing.T) {
			_, ok := claude.Stats{}.ComputedOn()
			assert.False(t, ok)
		})
	})
}
