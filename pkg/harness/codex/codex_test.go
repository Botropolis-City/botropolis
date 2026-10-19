package codex_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/harness/codex"
	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const rollout = `{"timestamp":"2026-09-17T07:00:00.000Z","type":"session_meta","payload":{"id":"019a3c2e-5a1b-7c3d-9e4f-0a1b2c3d4e5f","timestamp":"2026-09-17T07:00:00.000Z","cwd":"/home/avesta/workspaces/github/mCedar/cinders","originator":"codex_cli_rs","cli_version":"0.42.0","git":{"branch":"feat/queue","commit_hash":"abc"}}}
{"timestamp":"2026-09-17T07:00:01.000Z","type":"turn_context","payload":{"cwd":"/home/avesta/workspaces/github/mCedar/cinders","model":"gpt-5-codex","effort":"medium"}}
{"timestamp":"2026-09-17T07:00:02.000Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Fix the flaky queue test\nand add coverage"}]}}
{"timestamp":"2026-09-17T07:00:10.000Z","type":"response_item","payload":{"type":"function_call","name":"shell","arguments":"{}","call_id":"c1"}}
{"timestamp":"2026-09-17T07:00:12.000Z","type":"response_item","payload":{"type":"function_call_output","call_id":"c1","output":"ok"}}
{"timestamp":"2026-09-17T07:00:20.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":12000,"cached_input_tokens":9000,"output_tokens":800,"reasoning_output_tokens":300,"total_tokens":12800},"last_token_usage":{"input_tokens":7000,"cached_input_tokens":6000,"output_tokens":400},"model_context_window":272000}}}
{"timestamp":"2026-09-17T07:00:21.000Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Done."}]}}
{"timestamp":"2026-09-17T07:00:22.000Z","type":"event_msg","payload":{"type":"task_complete","last_agent_message":"Done."}}
`

var now = time.Date(2026, time.September, 17, 8, 0, 0, 0, time.UTC)

func writeRollout(t *testing.T, home, name, content string, modTime time.Time) string {
	t.Helper()
	dir := filepath.Join(home, "sessions", "2026", "09", "17")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	require.NoError(t, os.Chtimes(path, modTime, modTime))
	return path
}

func TestCodexLoader(t *testing.T) {
	t.Run("when a rollout finished an hour ago", func(t *testing.T) {
		home := t.TempDir()
		writeRollout(t, home, "rollout-2026-09-17T07-00-00-019a3c2e.jsonl", rollout, now.Add(-time.Hour))
		loader := codex.NewLoader(home)
		snapshot, err := loader.Load(now)
		require.NoError(t, err)
		require.Len(t, snapshot.Sessions, 1)
		s := snapshot.Sessions[0]

		t.Run("it should read the session id, cwd and branch", func(t *testing.T) {
			assert.Equal(t, "019a3c2e-5a1b-7c3d-9e4f-0a1b2c3d4e5f", s.ID)
			assert.Equal(t, "/home/avesta/workspaces/github/mCedar/cinders", s.CWD)
			assert.Equal(t, "feat/queue", s.Branch)
		})

		t.Run("it should be tagged as codex", func(t *testing.T) {
			assert.Equal(t, "codex", s.Harness)
		})

		t.Run("it should take the model from the turn context", func(t *testing.T) {
			assert.Equal(t, "gpt-5-codex", s.Model)
		})

		t.Run("it should title it with the first line of the first prompt", func(t *testing.T) {
			assert.Equal(t, "Fix the flaky queue test", s.Title)
		})

		t.Run("it should read usage from the last token count, splitting cached input", func(t *testing.T) {
			assert.Equal(t, int64(3000), s.Usage.Input)
			assert.Equal(t, int64(9000), s.Usage.CacheRead)
			assert.Equal(t, int64(800), s.Usage.Output)
			assert.Equal(t, int64(300), s.Usage.Thinking)
		})

		t.Run("it should take its heft from its exact token counts", func(t *testing.T) {
			assert.Equal(t, int64(3000+800+9000/10), s.Heft)
		})

		t.Run("it should not mark its heft a lower bound", func(t *testing.T) {
			assert.False(t, s.HeftPartial)
		})

		t.Run("it should report its hefts read", func(t *testing.T) {
			assert.True(t, snapshot.HeftsRead)
		})

		t.Run("it should measure context against the model's window", func(t *testing.T) {
			assert.Equal(t, int64(272000), s.ContextWindow)
			assert.InDelta(t, 100.0*7000/272000, s.ContextPercent, 1e-9)
		})

		t.Run("it should say the window is known, because the rollout reports it", func(t *testing.T) {
			assert.True(t, s.ContextKnown)
		})

		t.Run("it should be parked because the file is stale", func(t *testing.T) {
			assert.Equal(t, state.Resting, s.State)
			assert.False(t, s.Alive)
		})

		t.Run("it should leave the plant's power to Claude", func(t *testing.T) {
			assert.Empty(t, snapshot.Power.ByModel)
		})
	})

	t.Run("when a rollout was written seconds ago and the turn is complete", func(t *testing.T) {
		home := t.TempDir()
		writeRollout(t, home, "rollout-a.jsonl", rollout, now.Add(-10*time.Second))
		snapshot, err := codex.NewLoader(home).Load(now)
		require.NoError(t, err)

		t.Run("it should need you", func(t *testing.T) {
			assert.Equal(t, state.NeedsYou, snapshot.Sessions[0].State)
			assert.True(t, snapshot.Sessions[0].Alive)
		})
	})

	t.Run("when a rollout was written seconds ago mid tool call", func(t *testing.T) {
		home := t.TempDir()
		mid := rollout[:strings.Index(rollout, `{"timestamp":"2026-09-17T07:00:12.000Z"`)]
		writeRollout(t, home, "rollout-b.jsonl", mid, now.Add(-10*time.Second))
		snapshot, err := codex.NewLoader(home).Load(now)
		require.NoError(t, err)

		t.Run("it should be working", func(t *testing.T) {
			assert.Equal(t, state.Working, snapshot.Sessions[0].State)
		})
	})

	t.Run("when a rollout is thirty days old", func(t *testing.T) {
		home := t.TempDir()
		writeRollout(t, home, "rollout-old.jsonl", rollout, now.Add(-30*24*time.Hour))
		snapshot, err := codex.NewLoader(home).Load(now)
		require.NoError(t, err)

		t.Run("it should still be on the snapshot", func(t *testing.T) {
			assert.Len(t, snapshot.Sessions, 1)
		})
	})

	t.Run("when the codex home does not exist", func(t *testing.T) {
		snapshot, err := codex.NewLoader(filepath.Join(t.TempDir(), "none")).Load(now)

		t.Run("it should return an empty snapshot and no error", func(t *testing.T) {
			assert.NoError(t, err)
			assert.Empty(t, snapshot.Sessions)
		})
	})
}
