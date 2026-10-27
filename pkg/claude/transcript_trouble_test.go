package claude_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func toolFailed(ts, toolUseID string) string {
	return fmt.Sprintf(`{"type":"user","isSidechain":false,"timestamp":%q,"sessionId":%q,"toolUseResult":"<scrubbed>",`+
		`"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":%q,"is_error":true,"content":"<scrubbed>"}]}}`,
		ts, sid, toolUseID)
}

func interrupted(ts, text string) string {
	return fmt.Sprintf(`{"type":"user","isSidechain":false,"timestamp":%q,"sessionId":%q,`+
		`"message":{"role":"user","content":[{"type":"text","text":%q}]}}`, ts, sid, text)
}

// Item 280: the clinic lists a session whose tool calls keep failing,
// and one that stopped mid-turn, which an interrupt is not.
func TestTranscriptTailTrouble(t *testing.T) {
	const (
		t0 = "2026-10-26T19:50:00.000Z"
		t1 = "2026-10-26T19:50:10.000Z"
		t2 = "2026-10-26T19:50:20.000Z"
		t3 = "2026-10-26T19:50:30.000Z"
		t4 = "2026-10-26T19:50:40.000Z"
		t5 = "2026-10-26T19:50:50.000Z"
		t6 = "2026-10-26T19:51:00.000Z"
		t7 = "2026-10-26T19:51:10.000Z"
	)
	failing := []string{
		prompt(t0),
		toolCall("msg_01", t1, "toolu_01", "Bash"), toolFailed(t2, "toolu_01"),
		toolCall("msg_02", t3, "toolu_02", "Bash"), toolFailed(t4, "toolu_02"),
		toolCall("msg_03", t5, "toolu_03", "Bash"), toolFailed(t6, "toolu_03"),
	}

	for _, c := range []struct {
		scenario string
		lines    []string
		failed   int
	}{
		{"when the last three tool results on the main line are errors", failing, 3},
		{"when an error is followed by a result that succeeded",
			append(append([]string{}, failing...), toolCall("msg_04", t7, "toolu_04", "Bash"), toolResult(t7, "toolu_04")), 0},
		{"when a prompt follows the errors", append(append([]string{}, failing...), prompt(t7)), 0},
		{"when the errors are on a sidechain", []string{
			prompt(t0), toolCall("msg_01", t1, "toolu_01", "Task"),
			sidechain(toolCall("msg_s1", t2, "toolu_s1", "Bash")), sidechain(toolFailed(t3, "toolu_s1")),
			sidechain(toolCall("msg_s2", t4, "toolu_s2", "Bash")), sidechain(toolFailed(t5, "toolu_s2")),
			sidechain(toolCall("msg_s3", t6, "toolu_s3", "Bash")), sidechain(toolFailed(t7, "toolu_s3")),
		}, 0},
	} {
		t.Run(c.scenario, func(t *testing.T) {
			transcript := readTranscript(t, append([]string{userLine}, c.lines...)...)

			t.Run(fmt.Sprintf("it should count %d failed in a row", c.failed), func(t *testing.T) {
				assert.Equal(t, c.failed, transcript.Tail.FailedTools)
			})
		})
	}

	for _, c := range []struct {
		scenario    string
		last        string
		interrupted bool
	}{
		{"when the last user line is an interrupt", interrupted(t2, "[Request interrupted by user]"), true},
		{"when the last user line is an interrupt during a tool", interrupted(t2, "[Request interrupted by user for tool use]"), true},
		{"when the last user line is a prompt", prompt(t2), false},
		{"when the last user line is a tool result", toolResult(t2, "toolu_01"), false},
	} {
		t.Run(c.scenario, func(t *testing.T) {
			transcript := readTranscript(t, userLine, prompt(t0), toolCall("msg_01", t1, "toolu_01", "Bash"), c.last)

			t.Run(fmt.Sprintf("it should say interrupted is %t", c.interrupted), func(t *testing.T) {
				assert.Equal(t, c.interrupted, transcript.Tail.Interrupted)
			})
		})
	}
}
