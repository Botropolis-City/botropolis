package claude_test

import (
	"fmt"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"github.com/stretchr/testify/assert"
)

type usageSpec struct {
	id, ts     string
	sidechain  bool
	in, out    int
	cacheRead  int
	cacheWrite int
	thinking   int
}

func assistantWithUsage(u usageSpec) string {
	return fmt.Sprintf(`{"type":"assistant","isSidechain":%t,"timestamp":%q,"sessionId":%q,`+
		`"message":{"id":%q,"model":"claude-opus-5","role":"assistant","content":[{"type":"text","text":"x"}],`+
		`"usage":{"input_tokens":%d,"output_tokens":%d,"cache_read_input_tokens":%d,"cache_creation_input_tokens":%d,`+
		`"output_tokens_details":{"thinking_tokens":%d}}}}`,
		u.sidechain, u.ts, sid, u.id, u.in, u.out, u.cacheRead, u.cacheWrite, u.thinking)
}

const costStateLine = `{"type":"cost-state","sessionId":"` + sid + `","totalCostUSD":8.652295,"totalAPIDuration":713026,` +
	`"startTime":1789595715519,"modelUsage":{` +
	`"claude-haiku-4-5-20251001":{"inputTokens":2480,"outputTokens":31,"thinkingTokens":0,"cacheReadInputTokens":0,` +
	`"cacheCreationInputTokens":0,"webSearchRequests":0,"costUSD":0.002635},` +
	`"claude-opus-5[1m]":{"inputTokens":3008,"outputTokens":50567,"thinkingTokens":17251,"cacheReadInputTokens":6850790,` +
	`"cacheCreationInputTokens":394505,"webSearchRequests":0,"costUSD":8.64966}}}`

func TestReadTranscriptUsage(t *testing.T) {
	first := usageSpec{id: "msg_01", ts: "2026-09-16T19:50:40.000Z", in: 2, out: 625, cacheRead: 23373, cacheWrite: 43510, thinking: 325}
	second := usageSpec{id: "msg_02", ts: "2026-09-16T19:51:10.000Z", in: 5, out: 100, cacheRead: 66000, cacheWrite: 900, thinking: 40}

	t.Run("when one assistant message is split across two records", func(t *testing.T) {
		transcript := readTranscript(t, userLine, assistantWithUsage(first), assistantWithUsage(first))

		t.Run("it should count its usage once", func(t *testing.T) {
			assert.Equal(t, int64(625), transcript.Usage.Output)
		})

		t.Run("it should count one message", func(t *testing.T) {
			assert.Equal(t, 1, transcript.Usage.Messages)
		})
	})

	t.Run("when the file holds two assistant messages", func(t *testing.T) {
		transcript := readTranscript(t, userLine, assistantWithUsage(first), assistantWithUsage(second))

		t.Run("it should sum the input tokens", func(t *testing.T) {
			assert.Equal(t, int64(7), transcript.Usage.Input)
		})

		t.Run("it should sum the output tokens", func(t *testing.T) {
			assert.Equal(t, int64(725), transcript.Usage.Output)
		})

		t.Run("it should sum the cache read tokens", func(t *testing.T) {
			assert.Equal(t, int64(89373), transcript.Usage.CacheRead)
		})

		t.Run("it should sum the cache creation tokens", func(t *testing.T) {
			assert.Equal(t, int64(44410), transcript.Usage.CacheCreate)
		})

		t.Run("it should sum the thinking tokens", func(t *testing.T) {
			assert.Equal(t, int64(365), transcript.Usage.Thinking)
		})

		t.Run("it should count two messages", func(t *testing.T) {
			assert.Equal(t, 2, transcript.Usage.Messages)
		})

		t.Run("it should report the context size of the last message", func(t *testing.T) {
			assert.Equal(t, int64(5+66000+900), transcript.ContextTokens)
		})

		t.Run("it should remember the largest context seen", func(t *testing.T) {
			assert.Equal(t, int64(5+66000+900), transcript.MaxContext)
		})

		t.Run("it should bucket usage by the hour it happened", func(t *testing.T) {
			hour := time.Date(2026, time.September, 16, 19, 0, 0, 0, time.UTC).Unix() / 3600
			assert.Equal(t, int64(725), transcript.Hourly[hour].Output)
		})
	})

	t.Run("when the last assistant message is a sidechain", func(t *testing.T) {
		side := second
		side.sidechain = true
		transcript := readTranscript(t, userLine, assistantWithUsage(first), assistantWithUsage(side))

		t.Run("it should still add its usage to the totals", func(t *testing.T) {
			assert.Equal(t, int64(725), transcript.Usage.Output)
		})

		t.Run("it should report the context size of the last main-line message", func(t *testing.T) {
			assert.Equal(t, int64(2+23373+43510), transcript.ContextTokens)
		})
	})

	t.Run("when the file holds a cost-state record", func(t *testing.T) {
		transcript := readTranscript(t, userLine, assistantWithUsage(first), costStateLine)

		t.Run("it should report the total cost", func(t *testing.T) {
			assert.InDelta(t, 8.652295, transcript.Cost.TotalUSD, 1e-9)
		})

		t.Run("it should know the cost", func(t *testing.T) {
			assert.True(t, transcript.Cost.Known)
		})

		t.Run("it should report each model's cost", func(t *testing.T) {
			assert.InDelta(t, 8.64966, transcript.Cost.Models["claude-opus-5[1m]"].USD, 1e-9)
		})

		t.Run("it should report each model's tokens", func(t *testing.T) {
			assert.Equal(t, claude.Usage{
				Input:       3008,
				Output:      50567,
				CacheRead:   6850790,
				CacheCreate: 394505,
				Thinking:    17251,
			}, transcript.Cost.Models["claude-opus-5[1m]"].Usage)
		})
	})

	t.Run("when two cost-state records disagree", func(t *testing.T) {
		later := `{"type":"cost-state","sessionId":"` + sid + `","totalCostUSD":9.5,"modelUsage":{}}`
		transcript := readTranscript(t, userLine, costStateLine, later)

		t.Run("it should report the latest total cost", func(t *testing.T) {
			assert.InDelta(t, 9.5, transcript.Cost.TotalUSD, 1e-9)
		})
	})

	t.Run("when records carry timestamps", func(t *testing.T) {
		transcript := readTranscript(t, userLine, assistantWithUsage(first), assistantWithUsage(second))

		t.Run("it should report the first timestamp", func(t *testing.T) {
			assert.Equal(t, time.Date(2026, time.September, 16, 19, 50, 32, 231_000_000, time.UTC), transcript.FirstAt)
		})

		t.Run("it should report the last timestamp", func(t *testing.T) {
			assert.Equal(t, time.Date(2026, time.September, 16, 19, 51, 10, 0, time.UTC), transcript.LastAt)
		})
	})

	t.Run("when the file holds no cost-state record", func(t *testing.T) {
		transcript := readTranscript(t, userLine, assistantWithUsage(first))

		t.Run("it should not know the cost", func(t *testing.T) {
			assert.False(t, transcript.Cost.Known)
		})
	})

	t.Run("when the file holds no assistant records", func(t *testing.T) {
		transcript := readTranscript(t, userLine)

		t.Run("it should report zero usage", func(t *testing.T) {
			assert.Equal(t, claude.Usage{}, transcript.Usage)
		})

		t.Run("it should report zero context", func(t *testing.T) {
			assert.Zero(t, transcript.ContextTokens)
		})
	})
}
