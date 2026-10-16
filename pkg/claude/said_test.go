package claude_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
)

func TestReadSaid(t *testing.T) {
	const ts = "2026-10-16T09:00:00.000Z"
	typed := func(text string) string {
		return fmt.Sprintf(`{"type":"user","isSidechain":false,"timestamp":%q,"sessionId":%q,"cwd":"/src/kiln","gitBranch":"main",`+
			`"message":{"role":"user","content":%q}}`, ts, sid, text)
	}
	assistant := func(blocks string) string {
		return fmt.Sprintf(`{"type":"assistant","isSidechain":false,"timestamp":%q,"sessionId":%q,`+
			`"message":{"id":"msg_1","role":"assistant","content":[%s]}}`, ts, sid, blocks)
	}
	tool := func(name, input string) string {
		return assistant(fmt.Sprintf(`{"type":"tool_use","id":"toolu_1","name":%q,"input":%s}`, name, input))
	}
	passages := func(t *testing.T, lines ...string) []claude.Passage {
		t.Helper()
		said, err := claude.ReadSaid(writeTranscript(t, lines...))
		require.NoError(t, err)
		var out []claude.Passage
		for _, p := range said.Passages {
			out = append(out, claude.Passage{Kind: p.Kind, Text: p.Text})
		}
		return out
	}
	only := func(kind claude.SaidKind, text string) []claude.Passage {
		return []claude.Passage{{Kind: kind, Text: text}}
	}

	for _, c := range []struct {
		when  string
		it    string
		lines []string
		want  []claude.Passage
	}{
		{"when a transcript has a typed prompt", "it should keep it as a prompt",
			[]string{typed("why does the kiln overheat")}, only(claude.SaidPrompt, "why does the kiln overheat")},
		{"when a user record is a tool result", "it should leave it out",
			[]string{fmt.Sprintf(`{"type":"user","sessionId":%q,"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"thermocouple"},{"type":"text","text":"thermocouple read 900"}]}}`, sid)}, nil},
		{"when a user record is the CLI's own, marked meta", "it should leave it out",
			[]string{fmt.Sprintf(`{"type":"user","isMeta":true,"sessionId":%q,"message":{"role":"user","content":"Caveat: thermocouple"}}`, sid)}, nil},
		{"when a user record is the CLI's own, a summary of a compacted conversation", "it should leave it out",
			[]string{fmt.Sprintf(`{"type":"user","isCompactSummary":true,"sessionId":%q,"message":{"role":"user","content":"This session is being continued from a previous conversation"}}`, sid)}, nil},
		{"when a user record is the CLI's own, wrapped in a tag", "it should leave it out",
			[]string{typed("<command-name>/clear</command-name>")}, nil},
		{"when Claude replies with text", "it should keep it as a reply",
			[]string{assistant(`{"type":"text","text":"The thermocouple is loose."}`)}, only(claude.SaidReply, "The thermocouple is loose.")},
		{"when Claude runs a command", "it should keep the command",
			[]string{tool("Bash", `{"command":"make test-kiln","description":"run the kiln tests"}`)}, only(claude.SaidCommand, "make test-kiln")},
		{"when Claude reads a file", "it should keep the path",
			[]string{tool("Read", `{"file_path":"/src/kiln/heat.go"}`)}, only(claude.SaidPath, "/src/kiln/heat.go")},
		{"when Claude writes a file", "it should keep the path and not the content",
			[]string{tool("Write", `{"file_path":"/src/kiln/heat.go","content":"package kiln"}`)}, only(claude.SaidPath, "/src/kiln/heat.go")},
		{"when the session left a recap", "it should keep the recap",
			[]string{awaySummary(ts, "Fixing the kiln's thermocouple.")}, only(claude.SaidRecap, "Fixing the kiln's thermocouple.")},
		{"when the session has a title", "it should keep the title",
			[]string{fmt.Sprintf(`{"type":"ai-title","aiTitle":"Kiln overheating","sessionId":%q}`, sid)}, only(claude.SaidTitle, "Kiln overheating")},
		{"when a record is a subagent's", "it should leave it out",
			[]string{fmt.Sprintf(`{"type":"user","isSidechain":true,"sessionId":%q,"message":{"role":"user","content":"search the kiln"}}`, sid)}, nil},
		{"when a line does not parse", "it should skip it and keep reading",
			[]string{`{"type":"user",`, typed("after the broken line")}, only(claude.SaidPrompt, "after the broken line")},
	} {
		t.Run(c.when, func(t *testing.T) {
			got := passages(t, c.lines...)

			t.Run(c.it, func(t *testing.T) {
				assert.Equal(t, c.want, got)
			})
		})
	}
}

func TestReadSaidSession(t *testing.T) {
	t.Run("when a transcript names its directory and branch", func(t *testing.T) {
		said, err := claude.ReadSaid(writeTranscript(t, userLine))
		require.NoError(t, err)

		t.Run("it should keep them with the session", func(t *testing.T) {
			assert.Equal(t, [3]string{sid, "/home/avesta/workspaces/github/mCedar/cinders", "main"}, [3]string{said.SessionID, said.CWD, said.Branch})
		})
	})
}
