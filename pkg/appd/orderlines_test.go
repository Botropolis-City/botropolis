package appd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 90: the lines a group's order is read from stay in memory.
func TestOrderLinesAreNeverWritten(t *testing.T) {
	now := time.Date(2026, 9, 17, 8, 0, 0, 0, time.UTC)
	dead := state.Probes{Alive: func(int) bool { return false }, Attached: func(string) bool { return false }}
	const marker = "code.example.com/acme/lib/pull/4141"
	home := helpers.NewHome(t)
	for _, s := range []struct{ id, dir, at string }{
		{"11111111-0000-0000-0000-000000000000", "/w/lib", "2026-09-16T10:00:00.000Z"},
		{"22222222-0000-0000-0000-000000000000", "/w/api", "2026-09-16T14:00:00.000Z"},
	} {
		home.Transcript(s.id, s.dir, helpers.UserPrompt(s.id, s.dir, s.at), helpers.AssistantReply(s.id, "msg_"+s.id[:4], s.at),
			fmt.Sprintf(`{"type":"assistant","isSidechain":false,"timestamp":%q,"sessionId":%q,`+
				`"message":{"id":"msg_dep%s","model":"claude-opus-5","role":"assistant","stop_reason":"tool_use",`+
				`"content":[{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"git commit -m x\n\nDepends-On: https://%s"}}],`+
				`"usage":{"input_tokens":2,"output_tokens":5}}}`, s.at, s.id, s.id[:4], marker))
		at, _ := time.Parse(time.RFC3339, s.at)
		home.Prompt(s.id, s.dir, "PROJ-123 headers", at)
	}
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	snapshot, err := Harnesses(&config.Config{Home: home.Path}, dead, Paced).Load(now)
	require.NoError(t, err)

	t.Run("when a home whose sessions carry order lines is loaded", func(t *testing.T) {
		t.Run("it should read them", func(t *testing.T) {
			var lines int
			for _, s := range snapshot.Sessions {
				lines += len(s.OrderLines)
			}
			assert.Positive(t, lines)
		})

		var written, holding []string
		require.NoError(t, filepath.WalkDir(stateDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			written = append(written, path)
			data, err := os.ReadFile(path)
			if err == nil && strings.Contains(string(data), marker) {
				holding = append(holding, path)
			}
			return err
		}))

		t.Run("it should have written its own state, so the search below searched something", func(t *testing.T) {
			assert.NotEmpty(t, written)
		})

		t.Run("it should never write order lines anywhere", func(t *testing.T) {
			assert.Empty(t, holding)
		})
	})
}
