package claude_test

import (
	"os"
	"path/filepath"
	"testing"

	"botropolis.city/botropolis/pkg/claude"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	guideMeta = `{"agentType":"claude-code-guide","description":"<scrubbed>","toolUseId":"toolu_01CVLy",` +
		`"spawnDepth":1,"requestShape":"<scrubbed>","requestNonInteractive":true}`
	exploreMeta = `{"agentType":"Explore","description":"<scrubbed>","toolUseId":"toolu_01Expl","spawnDepth":2}`
)

func subagentLines() string {
	return sidechain(prompt("2026-09-16T19:59:56.509Z")) + "\n" +
		sidechain(assistantWithUsage(usageSpec{id: "msg_sub", ts: "2026-09-16T20:00:10.000Z", in: 7, out: 300, cacheRead: 5000})) + "\n"
}

func writeSubagents(t *testing.T, files map[string]string) string {
	t.Helper()
	project := t.TempDir()
	transcript := writeFile(t, project, sid+".jsonl", userLine+"\n")
	if files != nil {
		dir := filepath.Join(project, sid, "subagents")
		require.NoError(t, os.MkdirAll(dir, 0o700))
		for name, content := range files {
			writeFile(t, dir, name, content)
		}
	}
	return transcript
}

func TestReadSubagents(t *testing.T) {
	t.Run("when the transcript has two subagents with meta files", func(t *testing.T) {
		transcript := writeSubagents(t, map[string]string{
			"agent-a2f854e70.jsonl":     subagentLines(),
			"agent-a2f854e70.meta.json": guideMeta,
			"agent-a0ef1c9f4.jsonl":     subagentLines(),
			"agent-a0ef1c9f4.meta.json": exploreMeta,
		})
		subagents, skipped, err := claude.ReadSubagents(transcript)
		require.NoError(t, err)
		require.Len(t, subagents, 2)

		t.Run("it should return them sorted by path", func(t *testing.T) {
			assert.Equal(t, []string{"a0ef1c9f4", "a2f854e70"}, []string{subagents[0].AgentID, subagents[1].AgentID})
		})

		t.Run("it should read the agent type from the meta file", func(t *testing.T) {
			assert.Equal(t, "claude-code-guide", subagents[1].AgentType)
		})

		t.Run("it should read the tool use id from the meta file", func(t *testing.T) {
			assert.Equal(t, "toolu_01Expl", subagents[0].ToolUseID)
		})

		t.Run("it should read the spawn depth from the meta file", func(t *testing.T) {
			assert.Equal(t, 2, subagents[0].SpawnDepth)
		})

		t.Run("it should total the subagent's usage", func(t *testing.T) {
			assert.Equal(t, int64(300), subagents[0].Usage.Output)
		})

		t.Run("it should measure the subagent's context from its own messages", func(t *testing.T) {
			assert.Equal(t, int64(5007), subagents[0].ContextTokens)
		})

		t.Run("it should treat the subagent's sidechain as its main line for turn state", func(t *testing.T) {
			assert.Equal(t, claude.TurnAwaitingUser, subagents[0].Tail.Turn)
		})

		t.Run("it should count the subagent's prompts", func(t *testing.T) {
			assert.Equal(t, 1, subagents[0].Tail.Prompts)
		})

		t.Run("it should record the transcript path", func(t *testing.T) {
			assert.Equal(t, filepath.Join(filepath.Dir(transcript), sid, "subagents", "agent-a0ef1c9f4.jsonl"), subagents[0].Path)
		})

		t.Run("it should report nothing skipped", func(t *testing.T) {
			assert.Empty(t, skipped)
		})
	})

	t.Run("when a subagent has no meta file", func(t *testing.T) {
		transcript := writeSubagents(t, map[string]string{"agent-a2f854e70.jsonl": subagentLines()})
		subagents, skipped, err := claude.ReadSubagents(transcript)
		require.NoError(t, err)

		t.Run("it should still return the subagent", func(t *testing.T) {
			assert.Len(t, subagents, 1)
		})

		t.Run("it should leave the agent type empty", func(t *testing.T) {
			assert.Empty(t, subagents[0].AgentType)
		})

		t.Run("it should report nothing skipped", func(t *testing.T) {
			assert.Empty(t, skipped)
		})
	})

	t.Run("when a meta file is not valid JSON", func(t *testing.T) {
		transcript := writeSubagents(t, map[string]string{
			"agent-a2f854e70.jsonl":     subagentLines(),
			"agent-a2f854e70.meta.json": `{"agentType":`,
		})
		subagents, skipped, err := claude.ReadSubagents(transcript)
		require.NoError(t, err)

		t.Run("it should still return the subagent", func(t *testing.T) {
			assert.Len(t, subagents, 1)
		})

		t.Run("it should report the meta file as skipped", func(t *testing.T) {
			require.Len(t, skipped, 1)
			assert.Equal(t, filepath.Join(filepath.Dir(transcript), sid, "subagents", "agent-a2f854e70.meta.json"), skipped[0].Path)
		})
	})

	t.Run("when the transcript has no subagents directory", func(t *testing.T) {
		subagents, skipped, err := claude.ReadSubagents(writeSubagents(t, nil))

		t.Run("it should not return an error", func(t *testing.T) {
			assert.NoError(t, err)
		})

		t.Run("it should return no subagents", func(t *testing.T) {
			assert.Empty(t, subagents)
		})

		t.Run("it should report nothing skipped", func(t *testing.T) {
			assert.Empty(t, skipped)
		})
	})
}
