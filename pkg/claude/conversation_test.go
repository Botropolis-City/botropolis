package claude_test

import (
	"bytes"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
)

func TestReadConversation(t *testing.T) {
	const ts = "2026-10-16T09:00:00.000Z"
	typed := func(text string) string {
		return fmt.Sprintf(`{"type":"user","isSidechain":false,"timestamp":%q,"sessionId":%q,`+
			`"message":{"role":"user","content":%q}}`, ts, sid, text)
	}
	assistantIn := func(id, blocks string) string {
		return fmt.Sprintf(`{"type":"assistant","isSidechain":false,"timestamp":%q,"sessionId":%q,`+
			`"message":{"id":%q,"role":"assistant","content":[%s]}}`, ts, sid, id, blocks)
	}
	reply := func(id, text string) string {
		return assistantIn(id, fmt.Sprintf(`{"type":"text","text":%q}`, text))
	}
	tool := func(name, input string) string {
		return assistantIn("msg_tool", fmt.Sprintf(`{"type":"tool_use","id":"toolu_1","name":%q,"input":%s}`, name, input))
	}
	result := func(toolUseID string) string {
		return fmt.Sprintf(`{"type":"user","sessionId":%q,"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":%q,"content":"done"}]}}`, sid, toolUseID)
	}
	read := func(t *testing.T, skip, count int, lines ...string) claude.Conversation {
		t.Helper()
		conv, err := claude.ReadConversation(writeTranscript(t, lines...), skip, count)
		require.NoError(t, err)
		return conv
	}
	said := func(conv claude.Conversation) []claude.Message {
		var out []claude.Message
		for _, m := range conv.Messages {
			out = append(out, claude.Message{From: m.From, Text: m.Text})
		}
		return out
	}
	yours := func(text string) claude.Message { return claude.Message{From: claude.FromYou, Text: text} }
	claudes := func(text string) claude.Message { return claude.Message{From: claude.FromClaude, Text: text} }
	toolLine := func(text string) claude.Message { return claude.Message{From: claude.FromTool, Text: text} }

	for _, c := range []struct {
		when  string
		it    string
		lines []string
		want  []claude.Message
	}{
		{"when a transcript has typed prompts and replies", "it should keep them oldest first, each marked yours or Claude's",
			[]string{typed("why does the kiln overheat"), reply("msg_1", "The thermocouple is loose."), typed("tighten it")},
			[]claude.Message{yours("why does the kiln overheat"), claudes("The thermocouple is loose."), yours("tighten it")}},
		{"when a user record is a tool result", "it should leave it out",
			[]string{result("toolu_1")}, nil},
		{"when a user record is the CLI's own, marked meta", "it should leave it out",
			[]string{fmt.Sprintf(`{"type":"user","isMeta":true,"sessionId":%q,"message":{"role":"user","content":"Caveat: thermocouple"}}`, sid)}, nil},
		{"when a user record is the CLI's own, a summary of a compacted conversation", "it should leave it out",
			[]string{fmt.Sprintf(`{"type":"user","isCompactSummary":true,"sessionId":%q,"message":{"role":"user","content":"This session is being continued"}}`, sid)}, nil},
		{"when a user record is the CLI's own, wrapped in a tag", "it should leave it out",
			[]string{typed("<command-name>/clear</command-name>")}, nil},
		{"when a record is a subagent's", "it should leave it out",
			[]string{fmt.Sprintf(`{"type":"user","isSidechain":true,"sessionId":%q,"message":{"role":"user","content":"search the kiln"}}`, sid)}, nil},
		{"when one reply is written as several records of one message", "it should keep it as one turn",
			[]string{reply("msg_1", "The thermocouple is loose."), reply("msg_1", "Tightening it should do.")},
			[]claude.Message{claudes("The thermocouple is loose.\n\nTightening it should do.")}},
		{"when two replies follow one another as messages of their own", "it should keep them as two turns",
			[]string{reply("msg_1", "The thermocouple is loose."), reply("msg_2", "Tightening it should do.")},
			[]claude.Message{claudes("The thermocouple is loose."), claudes("Tightening it should do.")}},
		{"when Claude runs a command", "it should keep one line holding the command",
			[]string{tool("Bash", `{"command":"make test-kiln\n  --verbose","description":"run the kiln tests"}`)},
			[]claude.Message{toolLine("make test-kiln --verbose")}},
		{"when Claude reads a file", "it should keep one line holding the path",
			[]string{tool("Read", `{"file_path":"/src/kiln/heat.go"}`)}, []claude.Message{toolLine("/src/kiln/heat.go")}},
		{"when Claude writes a file", "it should keep one line holding the path, not the content",
			[]string{tool("Write", `{"file_path":"/src/kiln/heat.go","content":"package kiln"}`)}, []claude.Message{toolLine("/src/kiln/heat.go")}},
		{"when Claude edits a file", "it should keep one line holding the path, not the content",
			[]string{tool("Edit", `{"file_path":"/src/kiln/heat.go","old_string":"a","new_string":"b"}`)}, []claude.Message{toolLine("/src/kiln/heat.go")}},
		{"when Claude calls any other tool", "it should keep one line naming the tool",
			[]string{tool("WebSearch", `{"query":"kiln thermocouple"}`)}, []claude.Message{toolLine("WebSearch")}},
		{"when a line does not parse", "it should skip it and keep reading",
			[]string{`{"type":"user",`, typed("after the broken line")}, []claude.Message{yours("after the broken line")}},
	} {
		t.Run(c.when, func(t *testing.T) {
			got := said(read(t, 0, 20, c.lines...))

			t.Run(c.it, func(t *testing.T) {
				assert.Equal(t, c.want, got)
			})
		})
	}

	t.Run("when the session is waiting on AskUserQuestion", func(t *testing.T) {
		ask := assistantIn("msg_ask", `{"type":"tool_use","id":"toolu_ask","name":"AskUserQuestion","input":{"questions":[`+
			`{"question":"Which kiln should I fix first?","header":"Kiln","multiSelect":false,"options":[`+
			`{"label":"The glaze kiln","description":"It is hotter"},{"label":"The bisque kiln","description":"It is older"}]}]}}`)
		conv := read(t, 0, 20, typed("fix the kilns"), ask)

		t.Run("it should carry the question word for word", func(t *testing.T) {
			require.Len(t, conv.Questions, 1)
			assert.Equal(t, "Which kiln should I fix first?", conv.Questions[0].Text)
		})

		t.Run("it should carry its options", func(t *testing.T) {
			require.Len(t, conv.Questions, 1)
			assert.Equal(t, []string{"The glaze kiln", "The bisque kiln"}, conv.Questions[0].Options)
		})

		t.Run("and the question has been answered", func(t *testing.T) {
			answered := read(t, 0, 20, typed("fix the kilns"), ask, result("toolu_ask"))

			t.Run("it should carry no question", func(t *testing.T) {
				assert.Empty(t, answered.Questions)
			})
		})
	})

	t.Run("when Claude's last tool call has no result yet, as while it waits on your approval", func(t *testing.T) {
		call := assistantIn("msg_run", `{"type":"tool_use","id":"toolu_run","name":"Bash","input":{"command":"rm -rf build/\n  && make kiln","description":"rebuild"}}`)
		conv := read(t, 0, 20, typed("rebuild the kiln"), call)

		t.Run("it should carry the call as pending, word for word", func(t *testing.T) {
			assert.Equal(t, []claude.PendingCall{{Tool: "Bash", Input: "rm -rf build/\n  && make kiln"}}, conv.Pending)
		})

		t.Run("and the call has its result", func(t *testing.T) {
			done := read(t, 0, 20, typed("rebuild the kiln"), call, result("toolu_run"))

			t.Run("it should carry nothing pending", func(t *testing.T) {
				assert.Empty(t, done.Pending)
			})
		})

		t.Run("and the pending call is a question", func(t *testing.T) {
			asking := read(t, 0, 20, assistantIn("msg_ask", `{"type":"tool_use","id":"toolu_ask","name":"AskUserQuestion","input":{"questions":[{"question":"Which kiln?","options":[]}]}}`))

			t.Run("it should carry it as a question and not as a pending call", func(t *testing.T) {
				assert.Empty(t, asking.Pending)
			})
		})
	})

	numbered := func(n int) []string {
		var lines []string
		for i := 1; i <= n; i++ {
			lines = append(lines, typed(fmt.Sprintf("prompt %d", i)))
		}
		return lines
	}
	texts := func(conv claude.Conversation) []string {
		var out []string
		for _, m := range conv.Messages {
			out = append(out, m.Text)
		}
		return out
	}

	t.Run("when a page is asked for, skipping the newest n", func(t *testing.T) {
		conv := read(t, 2, 3, numbered(10)...)

		t.Run("it should return the count turns before them", func(t *testing.T) {
			assert.Equal(t, []string{"prompt 6", "prompt 7", "prompt 8"}, texts(conv))
		})
	})

	t.Run("when the page lies further back than one read of the file reaches", func(t *testing.T) {
		conv := read(t, 4000, 3, numbered(5000)...)

		t.Run("it should return the count turns before the skipped ones", func(t *testing.T) {
			assert.Equal(t, []string{"prompt 998", "prompt 999", "prompt 1000"}, texts(conv))
		})
	})

	t.Run("when more turns are asked for than the transcript holds", func(t *testing.T) {
		conv := read(t, 0, 20, numbered(4)...)

		t.Run("it should return what there is", func(t *testing.T) {
			assert.Equal(t, []string{"prompt 1", "prompt 2", "prompt 3", "prompt 4"}, texts(conv))
		})

		t.Run("it should say it reached the start", func(t *testing.T) {
			assert.True(t, conv.AtStart)
		})
	})

	t.Run("when the transcript holds far more than the page", func(t *testing.T) {
		data := []byte(strings.Join(numbered(5000), "\n") + "\n")
		r := &countingReader{data: data}
		_, err := claude.ReadConversationAt(r, int64(len(data)), 0, 5)
		require.NoError(t, err)

		t.Run("it should read less of the file than the whole", func(t *testing.T) {
			assert.Less(t, r.read.Load(), int64(len(data))/2)
		})
	})
}

type countingReader struct {
	data []byte
	read atomic.Int64
}

func (r *countingReader) ReadAt(p []byte, off int64) (int, error) {
	n, err := bytes.NewReader(r.data).ReadAt(p, off)
	r.read.Add(int64(n))
	return n, err
}
