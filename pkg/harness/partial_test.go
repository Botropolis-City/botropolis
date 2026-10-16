package harness_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/harness"
	"botropolis.city/botropolis/pkg/harness/codex"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 86: a stopped session read through a head and a tail window never
// reports its usage as a total, whichever harness it came from. The usage
// carries a partial marker, the way a cost carries costKnown (item 6).
// Codex's token counts are cumulative, so they stay exact through the
// window; Claude's are summed per message, so they are partial too.
func TestPartialMessageCount(t *testing.T) {
	now := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	const cwd = "/src/kiln"

	claudeHome := func(t *testing.T, replies int) harness.Snapshotter {
		t.Helper()
		home := helpers.NewHome(t)
		sid := "aaaaaaaa-0000-0000-0000-000000000001"
		lines := []string{helpers.UserPrompt(sid, cwd, "2026-10-14T09:00:00.000Z")}
		for i := 0; i < replies; i++ {
			lines = append(lines, helpers.AssistantReply(sid, fmt.Sprintf("msg_%04d", i), fmt.Sprintf("2026-10-14T09:%02d:%02d.000Z", (i/60)%60, i%60)))
		}
		home.Transcript(sid, cwd, lines...)
		return harness.NewClaude(state.NewLoader(home.Path, state.Probes{Alive: func(int) bool { return false }, Attached: func(string) bool { return false }}))
	}
	codexHome := func(t *testing.T, events int) harness.Snapshotter {
		t.Helper()
		home := t.TempDir()
		dir := filepath.Join(home, "sessions", "2026", "10", "15")
		require.NoError(t, os.MkdirAll(dir, 0o700))
		var b strings.Builder
		b.WriteString(`{"timestamp":"2026-10-15T09:00:00.000Z","type":"session_meta","payload":{"id":"019a3c2e-5a1b-7c3d-9e4f-0a1b2c3d4e5f","cwd":"/src/kiln"}}` + "\n")
		for i := 0; i < events; i++ {
			b.WriteString(`{"timestamp":"2026-10-15T09:00:01.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"cached_input_tokens":50,"output_tokens":10},"last_token_usage":{"input_tokens":100}}}}` + "\n")
		}
		path := filepath.Join(dir, "rollout-2026-10-15T09-00-00-019a3c2e.jsonl")
		require.NoError(t, os.WriteFile(path, []byte(b.String()), 0o600))
		old := now.Add(-24 * time.Hour)
		require.NoError(t, os.Chtimes(path, old, old))
		return codex.NewLoader(home)
	}
	usage := func(t *testing.T, h harness.Snapshotter) map[string]any {
		t.Helper()
		snapshot, err := h.Load(now)
		require.NoError(t, err)
		require.Len(t, snapshot.Sessions, 1)
		data, err := json.Marshal(snapshot.Sessions[0])
		require.NoError(t, err)
		var s struct {
			Usage map[string]any `json:"usage"`
		}
		require.NoError(t, json.Unmarshal(data, &s))
		return s.Usage
	}

	for _, c := range []struct {
		name  string
		build func(*testing.T, int) harness.Snapshotter
		small int
		large int
		// output is a session's whole output tokens for n records.
		output func(n int) float64
	}{
		{"claude", claudeHome, 3, 1000, func(n int) float64 { return float64(5 * n) }},
		{"codex", codexHome, 3, 2000, func(int) float64 { return 10 }},
	} {
		t.Run("when a "+c.name+" session was read through the window", func(t *testing.T) {
			u := usage(t, c.build(t, c.large))

			t.Run("it should mark its usage partial", func(t *testing.T) {
				assert.Equal(t, true, u["partial"])
			})

			t.Run("it should not report the message count as the total", func(t *testing.T) {
				assert.Less(t, u["messages"], float64(c.large))
			})

			t.Run("it should report no more output tokens than the whole session had", func(t *testing.T) {
				assert.LessOrEqual(t, u["output"], c.output(c.large))
			})
		})

		t.Run("when a "+c.name+" session was read whole", func(t *testing.T) {
			u := usage(t, c.build(t, c.small))

			t.Run("it should report the count", func(t *testing.T) {
				assert.Equal(t, float64(c.small), u["messages"])
			})

			t.Run("it should report the output tokens", func(t *testing.T) {
				assert.Equal(t, c.output(c.small), u["output"])
			})

			t.Run("it should carry no partial marker", func(t *testing.T) {
				assert.NotContains(t, u, "partial")
			})
		})
	}
}
