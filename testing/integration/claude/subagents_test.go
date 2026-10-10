package claude_test

import (
	"path/filepath"
	"testing"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/testing/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadSubagentsFromFixture(t *testing.T) {
	t.Run("when reading the subagents of every transcript in the sample fixture", func(t *testing.T) {
		home := helpers.FixtureHome(t, "sample")

		for _, session := range helpers.Manifest(t, "sample").Sessions {
			t.Run("and the transcript is "+session.SessionID, func(t *testing.T) {
				path := filepath.Join(home, ".claude", "projects", session.Project, session.SessionID+".jsonl")
				subagents, skipped, err := claude.ReadSubagents(path)
				require.NoError(t, err)

				t.Run("it should find one subagent per jsonl file the manifest counted", func(t *testing.T) {
					assert.Len(t, subagents, countGlob(t, filepath.Join(claude.SubagentsDir(path), "agent-*.jsonl")))
				})

				t.Run("it should skip nothing", func(t *testing.T) {
					assert.Empty(t, skipped)
				})

				for _, subagent := range subagents {
					t.Run("and the subagent is "+subagent.AgentID, func(t *testing.T) {
						t.Run("it should carry the parent's session id", func(t *testing.T) {
							assert.Equal(t, session.SessionID, subagent.SessionID)
						})

						t.Run("it should have an agent type", func(t *testing.T) {
							assert.NotEmpty(t, subagent.AgentType)
						})

						t.Run("it should link back to a tool use", func(t *testing.T) {
							assert.NotEmpty(t, subagent.ToolUseID)
						})

						t.Run("it should count at least one api message", func(t *testing.T) {
							assert.Positive(t, subagent.Usage.Messages)
						})

						t.Run("it should know whose turn it is", func(t *testing.T) {
							assert.NotEqual(t, claude.TurnUnknown, subagent.Tail.Turn)
						})
					})
				}
			})
		}
	})
}

func countGlob(t *testing.T, pattern string) int {
	t.Helper()
	matches, err := filepath.Glob(pattern)
	require.NoError(t, err)
	return len(matches)
}
