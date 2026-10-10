package claude_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"github.com/stretchr/testify/assert"
)

func prompt(ts string) string {
	return fmt.Sprintf(`{"type":"user","isSidechain":false,"timestamp":%q,"sessionId":%q,`+
		`"message":{"role":"user","content":"<scrubbed>"}}`, ts, sid)
}

func metaPrompt(ts string) string {
	return fmt.Sprintf(`{"type":"user","isSidechain":false,"isMeta":true,"timestamp":%q,"sessionId":%q,`+
		`"message":{"role":"user","content":[{"type":"text","text":"<scrubbed>"}]}}`, ts, sid)
}

func reply(id, ts string) string {
	return fmt.Sprintf(`{"type":"assistant","isSidechain":false,"timestamp":%q,"sessionId":%q,`+
		`"message":{"id":%q,"model":"claude-opus-5","role":"assistant","stop_reason":"end_turn",`+
		`"content":[{"type":"text","text":"<scrubbed>"}]}}`, ts, sid, id)
}

func thinkingBlock(id, ts string) string {
	return fmt.Sprintf(`{"type":"assistant","isSidechain":false,"timestamp":%q,"sessionId":%q,"apiBlockIndex":0,`+
		`"message":{"id":%q,"model":"claude-opus-5","role":"assistant","stop_reason":"tool_use",`+
		`"content":[{"type":"thinking","thinking":"<scrubbed>"}]}}`, ts, sid, id)
}

func toolCall(id, ts, toolUseID, name string) string {
	return fmt.Sprintf(`{"type":"assistant","isSidechain":false,"timestamp":%q,"sessionId":%q,"apiBlockIndex":1,`+
		`"message":{"id":%q,"model":"claude-opus-5","role":"assistant","stop_reason":"tool_use",`+
		`"content":[{"type":"tool_use","id":%q,"name":%q,"input":{}}]}}`, ts, sid, id, toolUseID, name)
}

func watchCall(id, ts, toolUseID, name string, stop bool) string {
	return fmt.Sprintf(`{"type":"assistant","isSidechain":false,"timestamp":%q,"sessionId":%q,"apiBlockIndex":1,`+
		`"message":{"id":%q,"model":"claude-opus-5","role":"assistant","stop_reason":"tool_use",`+
		`"content":[{"type":"tool_use","id":%q,"name":%q,"input":{"delaySeconds":600,"stop":%t}}]}}`, ts, sid, id, toolUseID, name, stop)
}

func toolResult(ts, toolUseID string) string {
	return fmt.Sprintf(`{"type":"user","isSidechain":false,"timestamp":%q,"sessionId":%q,"toolUseResult":"<scrubbed>",`+
		`"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":%q,"content":"<scrubbed>"}]}}`,
		ts, sid, toolUseID)
}

func sidechain(line string) string {
	return strings.Replace(line, `"isSidechain":false`, `"isSidechain":true`, 1)
}

func TestReadTranscriptTail(t *testing.T) {
	const (
		t0 = "2026-09-16T19:50:00.000Z"
		t1 = "2026-09-16T19:50:10.000Z"
		t2 = "2026-09-16T19:50:20.000Z"
		t3 = "2026-09-16T19:50:30.000Z"
		t4 = "2026-09-16T19:50:40.000Z"
	)

	t.Run("when the last main-line message is an assistant that ended its turn", func(t *testing.T) {
		transcript := readTranscript(t, userLine, prompt(t0), reply("msg_01", t1))

		t.Run("it should be awaiting the user", func(t *testing.T) {
			assert.Equal(t, claude.TurnAwaitingUser, transcript.Tail.Turn)
		})

		t.Run("it should have no pending tools", func(t *testing.T) {
			assert.Empty(t, transcript.Tail.PendingTools)
		})

		t.Run("it should record when the assistant last spoke", func(t *testing.T) {
			assert.Equal(t, time.Date(2026, time.September, 16, 19, 50, 10, 0, time.UTC), transcript.Tail.LastAssistantAt)
		})
	})

	t.Run("when the last message is a user prompt", func(t *testing.T) {
		transcript := readTranscript(t, userLine, prompt(t0), reply("msg_01", t1), prompt(t2))

		t.Run("it should be working", func(t *testing.T) {
			assert.Equal(t, claude.TurnWorking, transcript.Tail.Turn)
		})

		t.Run("it should record when the user last prompted", func(t *testing.T) {
			assert.Equal(t, time.Date(2026, time.September, 16, 19, 50, 20, 0, time.UTC), transcript.Tail.LastPromptAt)
		})
	})

	t.Run("when the last assistant message called a tool that has not returned", func(t *testing.T) {
		transcript := readTranscript(t, userLine, prompt(t0),
			thinkingBlock("msg_01", t1), toolCall("msg_01", t1, "toolu_01", "Bash"))

		t.Run("it should be working", func(t *testing.T) {
			assert.Equal(t, claude.TurnWorking, transcript.Tail.Turn)
		})

		t.Run("it should list the pending tool by name", func(t *testing.T) {
			assert.Equal(t, []string{"Bash"}, transcript.Tail.PendingTools)
		})

		t.Run("it should list the pending tool's id", func(t *testing.T) {
			assert.Equal(t, []string{"toolu_01"}, transcript.Tail.PendingToolIDs)
		})
	})

	t.Run("when the tool result has arrived", func(t *testing.T) {
		transcript := readTranscript(t, userLine, prompt(t0),
			thinkingBlock("msg_01", t1), toolCall("msg_01", t1, "toolu_01", "Bash"), toolResult(t2, "toolu_01"))

		t.Run("it should be working", func(t *testing.T) {
			assert.Equal(t, claude.TurnWorking, transcript.Tail.Turn)
		})

		t.Run("it should have no pending tools", func(t *testing.T) {
			assert.Empty(t, transcript.Tail.PendingTools)
		})
	})

	t.Run("when the last assistant message asked the user a question", func(t *testing.T) {
		transcript := readTranscript(t, userLine, prompt(t0), toolCall("msg_01", t1, "toolu_01", "AskUserQuestion"))

		t.Run("it should need input", func(t *testing.T) {
			assert.Equal(t, claude.TurnNeedsInput, transcript.Tail.Turn)
		})
	})

	t.Run("when a new assistant message follows an unanswered tool call", func(t *testing.T) {
		transcript := readTranscript(t, userLine, prompt(t0),
			toolCall("msg_01", t1, "toolu_01", "Bash"), prompt(t2), reply("msg_02", t3))

		t.Run("it should forget the abandoned tool", func(t *testing.T) {
			assert.Empty(t, transcript.Tail.PendingTools)
		})

		t.Run("it should be awaiting the user", func(t *testing.T) {
			assert.Equal(t, claude.TurnAwaitingUser, transcript.Tail.Turn)
		})
	})

	t.Run("when one assistant message calls two tools and one has returned", func(t *testing.T) {
		transcript := readTranscript(t, userLine, prompt(t0),
			toolCall("msg_01", t1, "toolu_01", "Read"), toolCall("msg_01", t1, "toolu_02", "Grep"), toolResult(t2, "toolu_01"))

		t.Run("it should list only the outstanding tool", func(t *testing.T) {
			assert.Equal(t, []string{"Grep"}, transcript.Tail.PendingTools)
		})
	})

	t.Run("when the user prompted three times and tools returned twice", func(t *testing.T) {
		transcript := readTranscript(t, prompt(t0), toolCall("msg_01", t1, "toolu_01", "Bash"), toolResult(t1, "toolu_01"),
			prompt(t2), metaPrompt(t2), toolCall("msg_02", t3, "toolu_02", "Bash"), toolResult(t3, "toolu_02"), prompt(t4))

		t.Run("it should count three prompts", func(t *testing.T) {
			assert.Equal(t, 3, transcript.Tail.Prompts)
		})
	})

	t.Run("when sidechain messages come after the main line ended its turn", func(t *testing.T) {
		transcript := readTranscript(t, prompt(t0), reply("msg_01", t1),
			sidechain(prompt(t2)), sidechain(toolCall("msg_02", t3, "toolu_09", "Bash")))

		t.Run("it should still be awaiting the user", func(t *testing.T) {
			assert.Equal(t, claude.TurnAwaitingUser, transcript.Tail.Turn)
		})

		t.Run("it should not count the sidechain prompt", func(t *testing.T) {
			assert.Equal(t, 1, transcript.Tail.Prompts)
		})
	})

	t.Run("when the file holds no messages", func(t *testing.T) {
		transcript := readTranscript(t, modeLine, bridgeLine)

		t.Run("it should have an unknown turn", func(t *testing.T) {
			assert.Equal(t, claude.TurnUnknown, transcript.Tail.Turn)
		})
	})
}

func TestTranscriptTailWaiting(t *testing.T) {
	const (
		t0 = "2026-09-16T19:50:00.000Z"
		t1 = "2026-09-16T19:50:10.000Z"
		t2 = "2026-09-16T19:50:20.000Z"
		t3 = "2026-09-16T19:50:30.000Z"
	)
	t.Run("when the assistant scheduled a wakeup and the turn ended", func(t *testing.T) {
		transcript := readTranscript(t, userLine, prompt(t0), watchCall("msg_01", t1, "toolu_01", "ScheduleWakeup", false), toolResult(t2, "toolu_01"))

		t.Run("it should be waiting on its own watch", func(t *testing.T) {
			assert.Equal(t, claude.TurnWaiting, transcript.Tail.Turn)
		})
	})

	t.Run("when the assistant spoke after scheduling", func(t *testing.T) {
		transcript := readTranscript(t, userLine, prompt(t0), watchCall("msg_01", t1, "toolu_01", "ScheduleWakeup", false), toolResult(t2, "toolu_01"), reply("msg_02", t3))

		t.Run("it should still be waiting", func(t *testing.T) {
			assert.Equal(t, claude.TurnWaiting, transcript.Tail.Turn)
		})
	})

	t.Run("when a Monitor is set", func(t *testing.T) {
		transcript := readTranscript(t, userLine, prompt(t0), watchCall("msg_01", t1, "toolu_01", "Monitor", false), toolResult(t2, "toolu_01"))

		t.Run("it should be waiting", func(t *testing.T) {
			assert.Equal(t, claude.TurnWaiting, transcript.Tail.Turn)
		})
	})

	t.Run("when the wakeup was stopped", func(t *testing.T) {
		transcript := readTranscript(t, userLine, prompt(t0), watchCall("msg_01", t1, "toolu_01", "ScheduleWakeup", true), toolResult(t2, "toolu_01"), reply("msg_02", t3))

		t.Run("it should hand the turn back to you", func(t *testing.T) {
			assert.Equal(t, claude.TurnAwaitingUser, transcript.Tail.Turn)
		})
	})

	t.Run("when a real prompt follows the scheduling", func(t *testing.T) {
		transcript := readTranscript(t, userLine, prompt(t0), watchCall("msg_01", t1, "toolu_01", "ScheduleWakeup", false), toolResult(t2, "toolu_01"), prompt(t3))

		t.Run("it should be working on the prompt", func(t *testing.T) {
			assert.Equal(t, claude.TurnWorking, transcript.Tail.Turn)
		})
	})

	t.Run("when a tool is still in flight after scheduling", func(t *testing.T) {
		transcript := readTranscript(t, userLine, prompt(t0), watchCall("msg_01", t1, "toolu_01", "ScheduleWakeup", false), toolResult(t2, "toolu_01"), toolCall("msg_02", t3, "toolu_02", "Bash"))

		t.Run("it should be working", func(t *testing.T) {
			assert.Equal(t, claude.TurnWorking, transcript.Tail.Turn)
		})
	})
}
