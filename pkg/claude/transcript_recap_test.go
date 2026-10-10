package claude_test

import (
	"fmt"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"github.com/stretchr/testify/assert"
)

func awaySummary(ts, text string) string {
	return fmt.Sprintf(`{"type":"system","subtype":"away_summary","content":%q,"isMeta":false,"timestamp":%q,"sessionId":%q}`,
		text, ts, sid)
}

func lastPrompt(text string) string {
	return fmt.Sprintf(`{"type":"last-prompt","lastPrompt":%q,"sessionId":%q}`, text, sid)
}

func stopHookSummary(ts string) string {
	return fmt.Sprintf(`{"type":"system","subtype":"stop_hook_summary","content":[{"x":1}],"timestamp":%q,"sessionId":%q}`, ts, sid)
}

func TestReadTranscriptRecap(t *testing.T) {
	const (
		t0 = "2026-10-06T10:00:00.000Z"
		t1 = "2026-10-06T10:05:00.000Z"
		t2 = "2026-10-06T10:10:00.000Z"
		t3 = "2026-10-06T10:15:00.000Z"
		t4 = "2026-10-06T10:20:00.000Z"
	)

	t.Run("when the transcript has no away summary", func(t *testing.T) {
		transcript := readTranscript(t, userLine, prompt(t0), reply("msg_01", t1))

		t.Run("it should have no recap", func(t *testing.T) {
			assert.Equal(t, claude.Recap{}, transcript.Recap)
		})
	})

	t.Run("when the transcript has several away summaries", func(t *testing.T) {
		transcript := readTranscript(t, userLine, prompt(t0), awaySummary(t1, "first"),
			prompt(t2), awaySummary(t3, "second"), reply("msg_01", t4))

		t.Run("it should keep the newest text", func(t *testing.T) {
			assert.Equal(t, "second", transcript.Recap.Text)
		})

		t.Run("it should keep when the newest was written", func(t *testing.T) {
			assert.Equal(t, time.Date(2026, time.October, 6, 10, 15, 0, 0, time.UTC), transcript.Recap.At)
		})

		t.Run("and no prompt follows the newest, it should count none since", func(t *testing.T) {
			assert.Equal(t, 0, transcript.Recap.PromptsSince)
		})
	})

	t.Run("when prompts follow the away summary", func(t *testing.T) {
		transcript := readTranscript(t, userLine, prompt(t0), awaySummary(t1, "recap"),
			prompt(t2), reply("msg_01", t3), metaPrompt(t3), prompt(t4))

		t.Run("it should count the real prompts since, not the meta ones", func(t *testing.T) {
			assert.Equal(t, 2, transcript.Recap.PromptsSince)
		})
	})

	t.Run("when a system record carries non-string content", func(t *testing.T) {
		transcript := readTranscript(t, userLine, prompt(t0), stopHookSummary(t1))

		t.Run("it should not count the record as malformed", func(t *testing.T) {
			assert.Equal(t, 0, transcript.Malformed)
		})
	})

	t.Run("when the transcript has last-prompt records", func(t *testing.T) {
		transcript := readTranscript(t, userLine, prompt(t0), lastPrompt("older"), prompt(t1), lastPrompt("newest"))

		t.Run("it should keep the newest", func(t *testing.T) {
			assert.Equal(t, "newest", transcript.LastPrompt)
		})
	})
}
