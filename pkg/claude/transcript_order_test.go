package claude_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/claude"
)

// Item 90: the lines a session's order can be read from, kept in memory.

func saidText(id, ts, text string) string {
	return fmt.Sprintf(`{"type":"assistant","isSidechain":false,"timestamp":%q,"sessionId":%q,`+
		`"message":{"id":%q,"model":"claude-opus-5","role":"assistant","stop_reason":"end_turn",`+
		`"content":[{"type":"text","text":%q}],"usage":{"input_tokens":2,"output_tokens":5}}}`, ts, sid, id, text)
}

func ranCommand(id, ts, command string) string {
	return fmt.Sprintf(`{"type":"assistant","isSidechain":false,"timestamp":%q,"sessionId":%q,`+
		`"message":{"id":%q,"model":"claude-opus-5","role":"assistant","stop_reason":"tool_use",`+
		`"content":[{"type":"tool_use","id":"toolu_%s","name":"Bash","input":{"command":%q}}],"usage":{"input_tokens":2,"output_tokens":5}}}`,
		ts, sid, id, id, command)
}

func TestOrderLines(t *testing.T) {
	const t0, t1 = "2026-10-06T10:00:00.000Z", "2026-10-06T10:05:00.000Z"

	for _, c := range []struct {
		when    string
		line    string
		want    claude.OrderLine
		without bool
	}{
		{"a Bash commit message carries a Depends-On line",
			ranCommand("msg_01", t1, "git commit -m \"Use the headers\n\nDepends-On: https://code.example.com/acme/lib/pull/41\""),
			claude.OrderLine{Text: "Depends-On: https://code.example.com/acme/lib/pull/41", Command: true}, false},
		{"Claude's text names a pull request URL",
			saidText("msg_01", t1, "Done.\nThis waits on https://code.example.com/acme/lib/pull/41 merging."),
			claude.OrderLine{Text: "This waits on https://code.example.com/acme/lib/pull/41 merging."}, false},
		{"a command pushes a tag",
			ranCommand("msg_01", t1, "git push github v1.4.0"),
			claude.OrderLine{Text: "git push github v1.4.0", Command: true}, false},
		{"a line names nothing",
			saidText("msg_01", t1, "The tests pass."),
			claude.OrderLine{}, true},
	} {
		t.Run("when "+c.when, func(t *testing.T) {
			transcript := readTranscript(t, userLine, prompt(t0), c.line)
			if c.without {
				t.Run("it should not keep it", func(t *testing.T) {
					assert.Empty(t, transcript.OrderLines)
				})
				return
			}

			t.Run("it should keep that line, marked by where it came from", func(t *testing.T) {
				var got []claude.OrderLine
				for _, l := range transcript.OrderLines {
					got = append(got, claude.OrderLine{Text: l.Text, Command: l.Command})
				}
				assert.Equal(t, []claude.OrderLine{c.want}, got)
			})
		})
	}

	t.Run("when more lines qualify than are kept", func(t *testing.T) {
		lines := []string{userLine, prompt(t0)}
		for i := range claude.OrderLinesKept + 5 {
			lines = append(lines, saidText(fmt.Sprintf("msg_%03d", i), t1, fmt.Sprintf("see acme/lib#%d", i)))
		}
		transcript := readTranscript(t, lines...)

		t.Run("it should keep the newest", func(t *testing.T) {
			assert.Len(t, transcript.OrderLines, claude.OrderLinesKept)
		})

		t.Run("the newest should be last", func(t *testing.T) {
			last := transcript.OrderLines[len(transcript.OrderLines)-1].Text
			assert.True(t, strings.HasSuffix(last, fmt.Sprintf("#%d", claude.OrderLinesKept+4)), last)
		})
	})
}
