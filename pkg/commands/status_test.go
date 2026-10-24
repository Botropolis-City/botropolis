package commands_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender(t *testing.T) {
	cwd := "/home/avesta/workspaces/github/auroq/botropolis"
	now := time.Date(2026, time.September, 17, 12, 0, 0, 0, time.UTC)
	row := func(t *testing.T, s state.Session) string {
		t.Helper()
		var out bytes.Buffer
		commands.Render(&out, state.Snapshot{At: now, Sessions: []state.Session{s}})
		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		require.Len(t, lines, 2)
		return lines[1]
	}

	t.Run("when a session is mid-tool on a project file", func(t *testing.T) {
		line := row(t, state.Session{ID: "a", CWD: cwd, State: state.Working, Tool: "Edit", Subject: cwd + "/pkg/city/city.go", StartedAt: now})

		t.Run("it should say what it is doing, relative to the project", func(t *testing.T) {
			assert.Contains(t, line, "Edit pkg/city/city.go")
		})
	})

	t.Run("when a session runs a long command", func(t *testing.T) {
		line := row(t, state.Session{ID: "a", CWD: cwd, State: state.Working, Tool: "Bash",
			Subject: "go test -count=1 ./... 2>&1 | grep -E FAIL | head -20", StartedAt: now})

		t.Run("it should clip the column", func(t *testing.T) {
			assert.Contains(t, line, "Bash go test -count=1 ./... 2>&…")
		})
	})

	t.Run("when a session is between tools", func(t *testing.T) {
		line := row(t, state.Session{ID: "a", CWD: cwd, State: state.NeedsYou, StartedAt: now})

		t.Run("it should dash the column", func(t *testing.T) {
			assert.Contains(t, line, "needs-you  -  ")
		})
	})

	t.Run("when a session runs on a dated model id", func(t *testing.T) {
		line := row(t, state.Session{ID: "a", CWD: cwd, State: state.Resting, Model: "claude-haiku-4-5-20251001", StartedAt: now})

		t.Run("it should print the short name in the MODEL column (item 181)", func(t *testing.T) {
			assert.Equal(t, [2]bool{true, false}, [2]bool{strings.Contains(line, " haiku-4-5 "), strings.Contains(line, "20251001")}, line)
		})
	})

	t.Run("when there are no sessions", func(t *testing.T) {
		var out bytes.Buffer
		commands.Render(&out, state.Snapshot{At: now})

		t.Run("it should say so", func(t *testing.T) {
			assert.Equal(t, "no sessions\n", out.String())
		})
	})
}

// Item 71 was filed as two live sessions missing from status and was a
// false positive: both were present, under titles taken from their
// transcripts rather than the names `claude agents` gives them. The two
// lists could only be reconciled by title, and titles are not unique —
// "What time is it" is two different mullet sessions in one run of
// `status --all`. Matching on a column that is not a key is what
// manufactured the bug, so status grew a key to match on.
func TestRenderJSON(t *testing.T) {
	now := time.Date(2026, time.September, 17, 12, 0, 0, 0, time.UTC)
	same := "the same title twice"
	snapshot := state.Snapshot{At: now, Sessions: []state.Session{
		{ID: "d7314bb1-6586-4392-b3b1-f5df4266017b", PID: 1165254, Title: same, State: state.NeedsYou},
		{ID: "ac752824-6bc4-479d-b9c4-842c2336b5a8", PID: 1698050, Title: same, State: state.Resting},
	}}
	decode := func(t *testing.T, s state.Snapshot) []map[string]any {
		t.Helper()
		var out bytes.Buffer
		require.NoError(t, commands.RenderJSON(&out, s))
		var rows []map[string]any
		require.NoError(t, json.Unmarshal(out.Bytes(), &rows))
		return rows
	}

	t.Run("when two sessions share a title", func(t *testing.T) {
		rows := decode(t, snapshot)

		t.Run("and they are told apart by the id the CLI also knows", func(t *testing.T) {
			t.Run("it should carry every session's id", func(t *testing.T) {
				assert.Equal(t, []any{"d7314bb1-6586-4392-b3b1-f5df4266017b", "ac752824-6bc4-479d-b9c4-842c2336b5a8"},
					[]any{rows[0]["id"], rows[1]["id"]})
			})
		})

		t.Run("and `claude agents` reports a pid rather than a title", func(t *testing.T) {
			t.Run("it should carry every session's pid", func(t *testing.T) {
				assert.Equal(t, []any{float64(1165254), float64(1698050)}, []any{rows[0]["pid"], rows[1]["pid"]})
			})
		})
	})

	t.Run("when a session runs on a dated model id", func(t *testing.T) {
		rows := decode(t, state.Snapshot{At: now, Sessions: []state.Session{{ID: "a", Model: "claude-haiku-4-5-20251001"}}})

		t.Run("it should keep the full id (item 181)", func(t *testing.T) {
			assert.Equal(t, "claude-haiku-4-5-20251001", rows[0]["model"])
		})
	})

	t.Run("when no session is present", func(t *testing.T) {
		t.Run("it should emit an empty array rather than null, so a reader can iterate it", func(t *testing.T) {
			var out bytes.Buffer
			require.NoError(t, commands.RenderJSON(&out, state.Snapshot{At: now}))
			assert.Equal(t, "[]", strings.TrimSpace(out.String()))
		})
	})
}
