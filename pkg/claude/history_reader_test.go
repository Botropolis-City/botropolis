package claude_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"botropolis.city/botropolis/pkg/claude"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func historyLine(session, text string, at int64) string {
	return fmt.Sprintf(`{"display":%q,"pastedContents":{},"timestamp":%d,"project":"/w/lib","sessionId":%q}`+"\n", text, at, session)
}

func TestHistoryReader(t *testing.T) {
	one, two := historyLine("a", "one", 1), historyLine("b", "two", 2)
	three, four := historyLine("a", "three", 3), historyLine("c", "four", 4)
	write := func(t *testing.T, path, body string) {
		require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	}
	appendTo := func(t *testing.T, path, body string) {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		require.NoError(t, err)
		_, err = f.WriteString(body)
		require.NoError(t, err)
		require.NoError(t, f.Close())
	}
	replace := func(t *testing.T, path, body string) {
		write(t, path+".new", body)
		require.NoError(t, os.Rename(path+".new", path))
	}

	t.Run("when a history has been read", func(t *testing.T) {
		cases := []struct {
			scenario string
			before   string
			change   func(t *testing.T, path string)
		}{
			{
				scenario: "and lines are appended",
				before:   one + two,
				change:   func(t *testing.T, path string) { appendTo(t, path, three+four) },
			},
			{
				scenario: "and its partial last line is completed",
				before:   one + three[:15],
				change:   func(t *testing.T, path string) { appendTo(t, path, three[15:]+four) },
			},
			{
				scenario: "and the file is replaced with a shorter one",
				before:   one + two + three,
				change:   func(t *testing.T, path string) { replace(t, path, four) },
			},
			{
				scenario: "and the file is rewritten in place, longer, with a different start",
				before:   one,
				change:   func(t *testing.T, path string) { write(t, path, two+three+four) },
			},
			{
				scenario: "and the file is rewritten in place to the same size with different content",
				before:   one + two,
				change:   func(t *testing.T, path string) { write(t, path, historyLine("c", "ONE", 1)+two) },
			},
			{
				scenario: "and the file did not exist, and then does",
				change:   func(t *testing.T, path string) { write(t, path, one+two) },
			},
		}
		for _, c := range cases {
			t.Run(c.scenario, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "history.jsonl")
				if c.before != "" {
					write(t, path, c.before)
				}
				reader := claude.NewHistoryReader()
				_, err := reader.Read(path)
				require.NoError(t, err)
				c.change(t, path)
				got, err := reader.Read(path)
				require.NoError(t, err)
				want, err := claude.ReadPromptHistory(path)
				require.NoError(t, err)

				t.Run("it should read what ReadPromptHistory reads", func(t *testing.T) {
					assert.Equal(t, want, got)
				})
			})
		}
	})

	t.Run("when lines are appended after a read", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "history.jsonl")
		write(t, path, one+two)
		want, err := claude.ReadPromptHistory(path)
		require.NoError(t, err)
		reader := claude.NewHistoryReader()
		first, err := reader.Read(path)
		require.NoError(t, err)
		appendTo(t, path, three+four)
		_, err = reader.Read(path)
		require.NoError(t, err)

		t.Run("it should leave the earlier result as it was", func(t *testing.T) {
			assert.Equal(t, want, first)
		})
	})
}
