package codex_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/harness/codex"
	"botropolis.city/botropolis/pkg/state"
)

// Item 86: a rollout is read through a head and a tail window, as
// Claude's catalogue reads a transcript, so a long history loads in a
// time bounded by how many rollouts there are rather than their size.
func TestRolloutWindow(t *testing.T) {
	lines := strings.Split(strings.TrimSpace(rollout), "\n")
	head, calls, tail := lines[:3], lines[3]+"\n"+lines[4]+"\n"+lines[5]+"\n", strings.Join(lines[5:], "\n")+"\n"
	padded := func(middle string) string {
		return strings.Join(head, "\n") + "\n" + middle + tail
	}
	load := func(t *testing.T, content string) state.Session {
		t.Helper()
		home := t.TempDir()
		writeRollout(t, home, "rollout-2026-09-17T07-00-00-019a3c2e.jsonl", content, now.Add(-2*time.Hour))
		snapshot, err := codex.NewLoader(home).Load(now)
		require.NoError(t, err)
		require.Len(t, snapshot.Sessions, 1)
		return snapshot.Sessions[0]
	}
	whole := load(t, rollout)
	large := strings.Repeat(calls, 3*codex.Window/len(calls))
	fields := func(s state.Session) map[string]any {
		return map[string]any{
			"cwd": s.CWD, "branch": s.Branch, "title": s.Title, "model": s.Model,
			"started": s.StartedAt, "last activity": s.LastActivity, "state": s.State, "turn": s.Turn,
			"input": s.Usage.Input, "cached": s.Usage.CacheRead, "output": s.Usage.Output,
			"context tokens": s.ContextTokens, "context window": s.ContextWindow,
		}
	}

	t.Run("when a rollout is smaller than the window", func(t *testing.T) {
		got := load(t, padded(calls))

		t.Run("it should be read whole, counting every event", func(t *testing.T) {
			assert.Equal(t, 2, got.Usage.Messages)
		})
	})

	t.Run("when a rollout is larger than the window", func(t *testing.T) {
		got := fields(load(t, padded(large)))
		for name, want := range fields(whole) {
			t.Run("it should read the "+name+" the full scan reads", func(t *testing.T) {
				assert.Equal(t, want, got[name])
			})
		}

		t.Run("and its session_meta line is longer than the window", func(t *testing.T) {
			meta := strings.Replace(head[0], `"originator"`, `"instructions":"`+strings.Repeat("x", 2*codex.Window)+`","originator"`, 1)
			s := load(t, meta+"\n"+strings.Join(head[1:], "\n")+"\n"+large+tail)

			t.Run("it should still read the session's id", func(t *testing.T) {
				assert.Equal(t, whole.ID, s.ID)
			})

			t.Run("it should still read its working directory", func(t *testing.T) {
				assert.Equal(t, whole.CWD, s.CWD)
			})
		})

		t.Run("and the model changed only in the middle", func(t *testing.T) {
			changed := strings.Replace(lines[1], "gpt-5-codex", "gpt-5", 1)
			s := load(t, padded(large[:len(large)/2]+changed+"\n"+large[len(large)/2:]))

			t.Run("it should take the head's model", func(t *testing.T) {
				assert.Equal(t, "gpt-5-codex", s.Model)
			})
		})

		t.Run("and its token counts are counted", func(t *testing.T) {
			s := load(t, padded(large))
			all := strings.Count(padded(large), `"type":"token_count"`)

			t.Run("it should count only the events inside the window, as Claude's catalogue does", func(t *testing.T) {
				assert.Less(t, s.Usage.Messages, all)
			})
		})
	})
}
