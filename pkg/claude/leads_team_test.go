package claude_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Item 253: a lead's own lines name no team in Claude Code 2.1.310; the
// result of the Agent call that spawned a teammate does. Shaped like
// item 105's recording, structure only.
func TestLeadsTeam(t *testing.T) {
	const spawn = `{"type":"assistant","isSidechain":false,"timestamp":"2026-10-25T10:00:00.000Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_A","name":"Agent","input":{"description":"…","name":"reader","model":"…","prompt":"…"},"caller":{"type":"direct"}}]}}`
	result := func(toolUseResult string) string {
		return `{"type":"user","isSidechain":false,"timestamp":"2026-10-25T10:00:01.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_A","content":[{"type":"text","text":"…"}]}]},"toolUseResult":` + toolUseResult + `}`
	}

	t.Run("when a lead's Agent result names the team it spawned a teammate into", func(t *testing.T) {
		tr := readTranscript(t, spawn, result(`{"status":"teammate_spawned","prompt":"…","agentId":"reader@session-2aa29be8","teammate_id":"reader@session-2aa29be8","agent_id":"reader@session-2aa29be8","name":"reader","team_name":"session-2aa29be8","is_splitpane":true,"plan_mode_required":false}`))

		t.Run("it should record the team it leads", func(t *testing.T) {
			assert.Equal(t, "session-2aa29be8", tr.LeadsTeam)
		})
	})

	t.Run("when a tool result is plain text", func(t *testing.T) {
		tr := readTranscript(t, spawn, result(`"…"`))

		t.Run("it should lead no team", func(t *testing.T) {
			assert.Empty(t, tr.LeadsTeam)
		})
	})

	t.Run("when a teammate's lines carry its team and its results name none", func(t *testing.T) {
		tr := readTranscript(t,
			`{"type":"user","teamName":"session-2aa29be8","agentName":"reader","timestamp":"2026-10-25T10:00:02.000Z","message":{"role":"user","content":"…"}}`,
			`{"type":"assistant","teamName":"session-2aa29be8","agentName":"reader","timestamp":"2026-10-25T10:00:03.000Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_B","name":"SendMessage","input":{"to":"team-lead","message":"…","summary":"…"}}]}}`,
			`{"type":"user","teamName":"session-2aa29be8","agentName":"reader","timestamp":"2026-10-25T10:00:04.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_B","content":"…"}]},"toolUseResult":{"success":true}}`)

		t.Run("it should lead no team", func(t *testing.T) {
			assert.Empty(t, tr.LeadsTeam)
		})
	})
}
