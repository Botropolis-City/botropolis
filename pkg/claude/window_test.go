package claude_test

import (
	"fmt"
	"strings"
	"testing"

	"botropolis.city/botropolis/pkg/claude"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func longTranscript(turns int) []string {
	lines := []string{userLine, modelLine}
	for i := 0; i < turns; i++ {
		ts := fmt.Sprintf("2026-09-16T20:%02d:00.000Z", i%60)
		lines = append(lines,
			prompt(ts),
			assistantWithUsage(usageSpec{id: fmt.Sprintf("msg_%03d", i), ts: ts, in: 10, out: 20}),
			`{"type":"ai-title","aiTitle":"Title `+fmt.Sprint(i)+`","sessionId":"`+sid+`"}`)
	}
	return lines
}

func TestReadTranscriptWindow(t *testing.T) {
	t.Run("when the file is smaller than the window", func(t *testing.T) {
		path := writeTranscript(t, longTranscript(3)...)
		full, err := claude.ReadTranscript(path)
		require.NoError(t, err)
		window, err := claude.ReadTranscriptWindow(path, 1<<20)
		require.NoError(t, err)

		t.Run("it should read the whole file", func(t *testing.T) {
			assert.Equal(t, full.Usage, window.Usage)
		})

		t.Run("it should not be marked partial", func(t *testing.T) {
			assert.False(t, window.Partial)
		})
	})

	t.Run("when the file is much larger than the window", func(t *testing.T) {
		lines := longTranscript(400)
		path := writeTranscript(t, lines...)
		window, err := claude.ReadTranscriptWindow(path, 4096)
		require.NoError(t, err)

		t.Run("it should be marked partial", func(t *testing.T) {
			assert.True(t, window.Partial)
		})

		t.Run("it should read the head for the cwd", func(t *testing.T) {
			assert.Equal(t, "/home/avesta/workspaces/github/mCedar/cinders", window.CWD)
		})

		t.Run("it should read the head for the model id", func(t *testing.T) {
			assert.Equal(t, "claude-opus-5[1m]", window.ModelID)
		})

		t.Run("it should read the tail for the latest title", func(t *testing.T) {
			assert.Equal(t, "Title 399", window.Title)
		})

		t.Run("it should read the tail for the last timestamp", func(t *testing.T) {
			assert.Equal(t, "2026-09-16T20:39:00Z", window.LastAt.Format("2006-01-02T15:04:05Z"))
		})

		t.Run("it should read the head for the first timestamp", func(t *testing.T) {
			assert.Equal(t, "2026-09-16T19:50:32.231Z", window.FirstAt.Format("2006-01-02T15:04:05.000Z"))
		})

		t.Run("it should know whose turn it is from the tail", func(t *testing.T) {
			assert.Equal(t, claude.TurnAwaitingUser, window.Tail.Turn)
		})

		t.Run("it should not count a line cut in half", func(t *testing.T) {
			assert.Zero(t, window.Malformed)
		})

		t.Run("it should count fewer messages than the file holds", func(t *testing.T) {
			assert.Less(t, window.Usage.Messages, 400)
		})
	})

	t.Run("when the file is a bridge stub", func(t *testing.T) {
		window, err := claude.ReadTranscriptWindow(writeTranscript(t, bridgeLine), 4096)
		require.NoError(t, err)

		t.Run("it should flag it", func(t *testing.T) {
			assert.True(t, window.IsBridgeStub)
		})
	})

	t.Run("when the file does not exist", func(t *testing.T) {
		_, err := claude.ReadTranscriptWindow(strings.Join([]string{t.TempDir(), "missing.jsonl"}, "/"), 4096)

		t.Run("it should return an error", func(t *testing.T) {
			assert.Error(t, err)
		})
	})
}
