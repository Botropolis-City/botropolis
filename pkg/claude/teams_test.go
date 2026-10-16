package claude_test

import (
	"os"
	"path/filepath"
	"testing"

	"botropolis.city/botropolis/pkg/claude"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const teamConfig = `{"name":"session-26c5d638","createdAt":1789634756489,"leadAgentId":"team-lead@session-26c5d638",` +
	`"leadSessionId":"26c5d638-15ae-4407-8a12-f0ccc00de69a","members":[` +
	`{"agentId":"team-lead@session-26c5d638","name":"team-lead","agentType":"team-lead","joinedAt":1789634756489,` +
	`"cwd":"/home/avesta/workspaces/github/auroq","subscriptions":[],"backendType":"in-process"},` +
	`{"agentId":"migrate-custom@session-26c5d638","name":"migrate-custom","agentType":"general-purpose","joinedAt":1789634800000,` +
	`"cwd":"/home/avesta/workspaces/github/mCedar/mullet","subscriptions":[],"backendType":"in-process"}]}`

func writeTeam(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, name), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name, "config.json"), []byte(content), 0o600))
}

func TestReadTeams(t *testing.T) {
	t.Run("when the teams directory holds one team", func(t *testing.T) {
		dir := t.TempDir()
		writeTeam(t, dir, "session-26c5d638", teamConfig)
		teams, skipped, err := claude.ReadTeams(dir)
		require.NoError(t, err)
		require.Len(t, teams, 1)

		t.Run("it should read the team name and lead session", func(t *testing.T) {
			assert.Equal(t, "session-26c5d638", teams[0].Name)
			assert.Equal(t, "26c5d638-15ae-4407-8a12-f0ccc00de69a", teams[0].LeadSessionID)
		})

		t.Run("it should read each member's name, type and cwd", func(t *testing.T) {
			assert.Equal(t, claude.TeamMember{Name: "migrate-custom", AgentID: "migrate-custom@session-26c5d638", AgentType: "general-purpose",
				CWD: "/home/avesta/workspaces/github/mCedar/mullet"}, teams[0].Members[1])
		})

		t.Run("it should find a member by name", func(t *testing.T) {
			member, ok := teams[0].Member("team-lead")
			require.True(t, ok)
			assert.Equal(t, "/home/avesta/workspaces/github/auroq", member.CWD)
		})

		t.Run("it should report nothing skipped", func(t *testing.T) {
			assert.Empty(t, skipped)
		})
	})

	t.Run("when a team config is malformed", func(t *testing.T) {
		dir := t.TempDir()
		writeTeam(t, dir, "good", teamConfig)
		writeTeam(t, dir, "bad", `{"name":`)
		teams, skipped, err := claude.ReadTeams(dir)
		require.NoError(t, err)

		t.Run("it should return the good one", func(t *testing.T) {
			assert.Len(t, teams, 1)
		})

		t.Run("it should report the bad one", func(t *testing.T) {
			require.Len(t, skipped, 1)
			assert.Equal(t, filepath.Join(dir, "bad", "config.json"), skipped[0].Path)
		})
	})

	t.Run("when the teams directory does not exist", func(t *testing.T) {
		teams, skipped, err := claude.ReadTeams(filepath.Join(t.TempDir(), "teams"))

		t.Run("it should return nothing and no error", func(t *testing.T) {
			assert.NoError(t, err)
			assert.Empty(t, teams)
			assert.Empty(t, skipped)
		})
	})
}

func TestTranscriptMessages(t *testing.T) {
	sendTo := func(id, to string) string {
		return `{"type":"assistant","isSidechain":false,"teamName":"session-26c5d638","agentName":"migrate-custom","timestamp":"2026-09-17T04:00:00.000Z","sessionId":"` + sid + `",` +
			`"message":{"id":"` + id + `","model":"claude-opus-5","role":"assistant","stop_reason":"tool_use",` +
			`"content":[{"type":"tool_use","id":"toolu_` + id + `","name":"SendMessage","input":{"to":"` + to + `","message":"<scrubbed>"}}]}}`
	}

	edits := func(id, path string) string {
		return `{"type":"assistant","isSidechain":false,"timestamp":"2026-09-17T04:00:00.000Z","sessionId":"` + sid + `",` +
			`"message":{"id":"` + id + `","model":"claude-opus-5","role":"assistant","stop_reason":"tool_use",` +
			`"content":[{"type":"tool_use","id":"toolu_` + id + `","name":"Edit","input":{"file_path":"` + path + `","old_string":"a","new_string":"b"}}]}}`
	}

	t.Run("when the transcript edits files in another project", func(t *testing.T) {
		transcript := readTranscript(t, userLine,
			edits("msg_10", "/home/avesta/workspaces/github/auroq/botropolis/pkg/city/city.go"),
			edits("msg_11", "/home/avesta/workspaces/github/auroq/botropolis/pkg/city/city.go"),
			edits("msg_12", "/home/avesta/workspaces/github/mCedar/cinders/main.go"))

		t.Run("it should count the foreign files touched", func(t *testing.T) {
			assert.Equal(t, map[string]int{"/home/avesta/workspaces/github/auroq/botropolis/pkg/city/city.go": 2}, transcript.Touches)
		})
	})

	uses := func(id, tool, key, path string) string {
		return `{"type":"assistant","isSidechain":false,"timestamp":"2026-09-17T04:00:00.000Z","sessionId":"` + sid + `",` +
			`"message":{"id":"` + id + `","model":"claude-opus-5","role":"assistant","stop_reason":"tool_use",` +
			`"content":[{"type":"tool_use","id":"toolu_` + id + `","name":"` + tool + `","input":{"` + key + `":"` + path + `"}}]}}`
	}
	for _, tool := range []struct{ name, key string }{
		{"Read", "file_path"}, {"Edit", "file_path"}, {"MultiEdit", "file_path"}, {"Write", "file_path"}, {"NotebookEdit", "notebook_path"},
	} {
		t.Run("when the transcript uses "+tool.name+" on a file in another project", func(t *testing.T) {
			path := "/home/avesta/workspaces/github/auroq/botropolis/notes.ipynb"
			transcript := readTranscript(t, userLine, uses("msg_20", tool.name, tool.key, path))

			t.Run("it should count it as a crossing", func(t *testing.T) {
				assert.Equal(t, map[string]int{path: 1}, transcript.Touches)
			})
		})
	}

	t.Run("when the transcript sends messages to teammates", func(t *testing.T) {
		transcript := readTranscript(t, userLine, sendTo("m1", "team-lead"), sendTo("m2", "team-lead"), sendTo("m3", "branch-a"))

		t.Run("it should read the team name", func(t *testing.T) {
			assert.Equal(t, "session-26c5d638", transcript.TeamName)
		})

		t.Run("it should read the agent name", func(t *testing.T) {
			assert.Equal(t, "migrate-custom", transcript.AgentName)
		})

		t.Run("it should count messages per recipient", func(t *testing.T) {
			assert.Equal(t, map[string]int{"team-lead": 2, "branch-a": 1}, transcript.Messages)
		})
	})

	t.Run("when the transcript sends nothing", func(t *testing.T) {
		transcript := readTranscript(t, userLine, assistantLine)

		t.Run("it should leave the map empty", func(t *testing.T) {
			assert.Empty(t, transcript.Messages)
		})
	})
}
