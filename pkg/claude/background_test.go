package claude_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Item 89: an agent launched in the background returns at once, so the
// parent never waits on it; the transcript records which calls those were.
func TestBackgroundAgents(t *testing.T) {
	const call = `{"type":"assistant","timestamp":"2026-10-09T20:13:22.528Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_A","name":"Agent","input":{}}]}}`
	result := func(extra string) string {
		return `{"type":"user","timestamp":"2026-10-09T20:13:22.546Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_A","content":"done"}]}` + extra + `}`
	}

	t.Run("when a tool result says an agent was launched in the background", func(t *testing.T) {
		tr := readTranscript(t, call, result(`,"toolUseResult":{"isAsync":true,"status":"async_launched","agentId":"a1"}`))

		t.Run("it should record that call as launched in the background", func(t *testing.T) {
			assert.Equal(t, map[string]bool{"toolu_A": true}, tr.Background)
		})
	})

	t.Run("when a tool result is an ordinary one", func(t *testing.T) {
		tr := readTranscript(t, call, result(`,"toolUseResult":"plain text"`))

		t.Run("it should record nothing", func(t *testing.T) {
			assert.Empty(t, tr.Background)
		})
	})
}
