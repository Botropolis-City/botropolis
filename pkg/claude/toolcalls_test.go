package claude_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Item 89: the main thread's tool calls are counted, each once, so the
// map can pulse for every one however short it was.
func TestToolCalls(t *testing.T) {
	call := func(id string, sidechain bool) string {
		side := "false"
		if sidechain {
			side = "true"
		}
		return `{"type":"assistant","isSidechain":` + side + `,"timestamp":"2026-10-17T12:00:00.000Z","message":{"id":"m-` + id + `","role":"assistant","content":[{"type":"tool_use","id":"` + id + `","name":"Read","input":{}}]}}`
	}

	t.Run("when the main thread makes three tool calls", func(t *testing.T) {
		tr := readTranscript(t, call("a", false), call("b", false), call("c", false))

		t.Run("it should count three", func(t *testing.T) {
			assert.Equal(t, 3, tr.ToolCalls)
		})
	})

	t.Run("when the same tool call appears on two lines", func(t *testing.T) {
		tr := readTranscript(t, call("a", false), call("a", false))

		t.Run("it should count it once", func(t *testing.T) {
			assert.Equal(t, 1, tr.ToolCalls)
		})
	})

	t.Run("when one message's two tool calls are on two lines that share its id", func(t *testing.T) {
		line := func(id string) string {
			return `{"type":"assistant","timestamp":"2026-10-17T12:00:00.000Z","message":{"id":"m-one","role":"assistant","content":[{"type":"tool_use","id":"` + id + `","name":"Read","input":{}}]}}`
		}
		tr := readTranscript(t, line("a"), line("b"))

		t.Run("it should count both", func(t *testing.T) {
			assert.Equal(t, 2, tr.ToolCalls)
		})
	})

	t.Run("when a call is on a sidechain", func(t *testing.T) {
		tr := readTranscript(t, call("a", false), call("b", true))

		t.Run("it should not count it", func(t *testing.T) {
			assert.Equal(t, 1, tr.ToolCalls)
		})
	})
}
