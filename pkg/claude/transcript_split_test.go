package claude_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/claude"
)

// Item 107: Claude Code writes one message's blocks on separate lines that
// share its id. The usage and attributions belong to the message, so they
// count once; every line's blocks are read.
func TestSplitMessage(t *testing.T) {
	const at = "2026-09-17T04:00:00.000Z"
	const foreign = "/home/avesta/workspaces/github/auroq/botropolis/pkg/city/city.go"

	line := func(attribution, block string) string {
		return `{"type":"assistant","isSidechain":false,"timestamp":"` + at + `","sessionId":"` + sid + `",` + attribution +
			`"message":{"id":"msg_split","model":"claude-opus-5","role":"assistant","stop_reason":"tool_use",` +
			`"content":[` + block + `],"usage":{"input_tokens":2,"output_tokens":5}}}`
	}
	const thinking = `{"type":"thinking","thinking":"","signature":"x"}`
	tool := func(id, name, input string) string {
		return `{"type":"tool_use","id":"` + id + `","name":"` + name + `","input":` + input + `}`
	}
	split := func(attribution, second string) claude.Transcript {
		return readTranscript(t, userLine, line(attribution, thinking), line(attribution, second))
	}

	t.Run("when one message's blocks are split across two lines that share its id", func(t *testing.T) {
		for _, c := range []struct {
			and, should string
			block       string
			got         func(claude.Transcript) any
			want        any
		}{
			{"the second is a Read of a file in another project", "count the touch",
				tool("toolu_a", "Read", `{"file_path":"`+foreign+`"}`),
				func(tr claude.Transcript) any { return tr.Touches }, map[string]int{foreign: 1}},
			{"the second is a SendMessage", "count the message to its recipient",
				tool("toolu_a", "SendMessage", `{"to":"team-lead","message":"<scrubbed>"}`),
				func(tr claude.Transcript) any { return tr.Messages }, map[string]int{"team-lead": 1}},
			{"the second is a Bash command naming an order", "keep the order line",
				tool("toolu_a", "Bash", `{"command":"git push github v1.4.0"}`),
				orderTexts, []string{"git push github v1.4.0"}},
			{"the second is text naming an order", "keep the order line",
				`{"type":"text","text":"This waits on acme/lib#41."}`,
				orderTexts, []string{"This waits on acme/lib#41."}},
			{"the second is a tool call", "count the tool call",
				tool("toolu_a", "Read", `{}`),
				func(tr claude.Transcript) any { return tr.ToolCalls }, 1},
		} {
			t.Run("and "+c.and, func(t *testing.T) {
				tr := split("", c.block)

				t.Run("it should "+c.should, func(t *testing.T) {
					assert.Equal(t, c.want, c.got(tr))
				})
			})
		}

		t.Run("it should count the usage once", func(t *testing.T) {
			assert.Equal(t, 1, split("", tool("toolu_a", "Read", `{}`)).Usage.Messages)
		})

		t.Run("and both lines carry an MCP server attribution", func(t *testing.T) {
			tr := split(`"attributionMcpServer":"github",`, tool("toolu_a", "mcp__github__get_issue", `{}`))

			t.Run("it should count one MCP call", func(t *testing.T) {
				assert.Equal(t, map[string]int{"github": 1}, tr.MCPCalls)
			})
		})

		t.Run("and both lines carry a skill attribution", func(t *testing.T) {
			tr := split(`"attributionSkill":"roadmap-add",`, tool("toolu_a", "Bash", `{"command":"ls"}`))

			t.Run("it should count one skill call", func(t *testing.T) {
				assert.Equal(t, map[string]int{"roadmap-add": 1}, tr.SkillCalls)
			})
		})
	})

	t.Run("when the same tool call appears on two lines that share an id", func(t *testing.T) {
		read := tool("toolu_a", "Read", `{"file_path":"`+foreign+`"}`)
		tr := readTranscript(t, userLine, line("", read), line("", read))

		t.Run("it should count its touch once", func(t *testing.T) {
			assert.Equal(t, map[string]int{foreign: 1}, tr.Touches)
		})
	})
}

func orderTexts(tr claude.Transcript) any {
	var texts []string
	for _, l := range tr.OrderLines {
		texts = append(texts, l.Text)
	}
	return texts
}
