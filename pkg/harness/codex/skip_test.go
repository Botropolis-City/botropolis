package codex_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/harness/codex"
	"botropolis.city/botropolis/pkg/measure"
	"botropolis.city/botropolis/pkg/state"
)

// Item 93: a line the session takes nothing from is not decoded, so a
// heavy history's first load decodes only the lines the map uses.
func TestDecodingOnlyWhatIsUsed(t *testing.T) {
	lines := strings.Split(strings.TrimSpace(rollout), "\n")
	reasoning := `{"timestamp":"2026-09-17T07:00:11.000Z","type":"response_item","payload":{"type":"reasoning","summary":[],"content":null,"encrypted_content":"QUJDREVGR0g="}}`
	output := func(ts, text string) string {
		return `{"timestamp":"` + ts + `","type":"response_item","payload":{"type":"function_call_output","call_id":"c2","output":"` + text + `"}}`
	}
	load := func(t *testing.T, content string) (state.Session, int64) {
		t.Helper()
		home := t.TempDir()
		writeRollout(t, home, "rollout-2026-09-17T07-00-00-019a3c2e.jsonl", content, now.Add(-time.Hour))
		before := measure.Read(measure.RolloutLinesDecoded)
		snapshot, err := codex.NewLoader(home).Load(now)
		require.NoError(t, err)
		require.Len(t, snapshot.Sessions, 1)
		return snapshot.Sessions[0], measure.Read(measure.RolloutLinesDecoded) - before
	}

	t.Run("when a rollout holds tool outputs and reasoning between the lines the session uses", func(t *testing.T) {
		used := append(append([]string{}, lines[:4]...), lines[5:]...)
		var padded []string
		for _, l := range lines {
			padded = append(padded, l, reasoning, output("2026-09-17T07:00:11.500Z", `plenty of output\n`))
		}
		padded = append(padded, output("2026-09-17T07:00:30.000Z", "late"))
		got, decoded := load(t, strings.Join(padded, "\n")+"\n")
		whole, _ := load(t, rollout)

		t.Run("it should decode only the lines the session uses", func(t *testing.T) {
			assert.Equal(t, int64(len(used)), decoded)
		})

		t.Run("it should take its last activity from its last line, whatever that line's kind", func(t *testing.T) {
			assert.Equal(t, time.Date(2026, time.September, 17, 7, 0, 30, 0, time.UTC), got.LastActivity.UTC())
		})

		fields := func(s state.Session) map[string]any {
			return map[string]any{
				"id": s.ID, "cwd": s.CWD, "branch": s.Branch, "title": s.Title, "model": s.Model,
				"started": s.StartedAt, "state": s.State, "turn": s.Turn,
				"input": s.Usage.Input, "cached": s.Usage.CacheRead, "output": s.Usage.Output,
				"thinking": s.Usage.Thinking, "messages": s.Usage.Messages,
				"context tokens": s.ContextTokens, "context window": s.ContextWindow,
			}
		}
		want, have := fields(whole), fields(got)
		for name := range want {
			t.Run("it should read the "+name+" the full decode reads", func(t *testing.T) {
				assert.Equal(t, want[name], have[name])
			})
		}
	})

	t.Run("when a tool output's text quotes a token_count record", func(t *testing.T) {
		quoted := output("2026-09-17T07:00:11.000Z", `{\"type\":\"event_msg\",\"payload\":{\"type\":\"token_count\",\"info\":{\"total_token_usage\":{\"output_tokens\":99999}}}}`)
		got, _ := load(t, strings.Join(lines[:5], "\n")+"\n"+quoted+"\n"+strings.Join(lines[5:], "\n")+"\n")

		t.Run("it should not count it as a token count", func(t *testing.T) {
			assert.Equal(t, 1, got.Usage.Messages)
		})
	})

	t.Run("when a line the session uses is not in Codex's compact form", func(t *testing.T) {
		spaced := `{"timestamp": "2026-09-17T07:00:20.000Z", "type": "event_msg", "payload": {"type": "token_count", "info": {"total_token_usage": {"input_tokens": 12000, "cached_input_tokens": 9000, "output_tokens": 800}}}}`
		got, _ := load(t, strings.Join(lines[:5], "\n")+"\n"+spaced+"\n"+strings.Join(lines[6:], "\n")+"\n")

		t.Run("it should still be decoded and counted", func(t *testing.T) {
			assert.Equal(t, int64(800), got.Usage.Output)
		})
	})
}
