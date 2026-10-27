package state_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/state"
)

// Item 280: the clinic lists a session that stopped mid-turn, and one
// whose tool calls keep failing.
func TestTrouble(t *testing.T) {
	const (
		toolCall = `{"type":"assistant","isSidechain":false,"timestamp":"2026-09-16T19:51:00.000Z","sessionId":"` + sidA + `",` +
			`"message":{"id":"msg_02","model":"claude-opus-5[1m]","role":"assistant","stop_reason":"tool_use",` +
			`"content":[{"type":"tool_use","id":"toolu_01","name":"Bash","input":{}}]}}
`
		failed = `{"type":"user","isSidechain":false,"timestamp":"2026-09-16T19:51:05.000Z","sessionId":"` + sidA + `",` +
			`"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_01","is_error":true,"content":"exit 1"}]}}
`
		interrupt = `{"type":"user","isSidechain":false,"timestamp":"2026-09-16T19:51:06.000Z","sessionId":"` + sidA + `",` +
			`"message":{"role":"user","content":[{"type":"text","text":"[Request interrupted by user for tool use]"}]}}
`
	)
	transcript := func(home string) string {
		return filepath.Join(home, ".claude", "projects", "-home-avesta-workspaces-github-mCedar-cinders", sidA+".jsonl")
	}
	load := func(t *testing.T, probes state.Probes, tail string) state.Session {
		t.Helper()
		home := writeHome(t, 4242)
		appendFile(t, transcript(home), tail)
		snapshot, err := state.Load(home, probes, now)
		require.NoError(t, err)
		require.Len(t, snapshot.Sessions, 1)
		return snapshot.Sessions[0]
	}

	for _, c := range []struct {
		scenario string
		probes   state.Probes
		tail     string
		stopped  bool
	}{
		{"when a session's process is gone with its turn unfinished", dead, toolCall, true},
		{"when it is gone after the user interrupted the turn", dead, toolCall + interrupt, false},
		{"when it is gone after its turn ended", dead, "", false},
		{"when its process is alive mid-turn", alive, toolCall, false},
	} {
		t.Run(c.scenario, func(t *testing.T) {
			s := load(t, c.probes, c.tail)

			t.Run("it should say whether it stopped mid-turn", func(t *testing.T) {
				assert.Equal(t, c.stopped, s.StoppedMidTurn)
			})
		})
	}

	t.Run("when its last three tool calls failed", func(t *testing.T) {
		s := load(t, alive, toolCall+failed+toolCall+failed+toolCall+failed)

		t.Run("it should carry the streak onto the session", func(t *testing.T) {
			assert.Equal(t, 3, s.FailedTools)
		})
	})
}
