package claude_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadPromptHistory(t *testing.T) {
	write := func(t *testing.T, body string) string {
		path := filepath.Join(t.TempDir(), "history.jsonl")
		require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
		return path
	}

	t.Run("when the history holds prompts from several sessions", func(t *testing.T) {
		path := write(t, `{"display":"look at PROJ-1","pastedContents":{},"timestamp":1792131808242,"project":"/w/lib","sessionId":"a"}
{"display":"and the docs","pastedContents":{},"timestamp":1792131816410,"project":"/w/app","sessionId":"b"}
{"display":"[Pasted text #1]","pastedContents":{"1":{"id":1,"type":"text","content":"the plan"}},"timestamp":1792131819105,"project":"/w/lib","sessionId":"a"}
`)
		history, err := claude.ReadPromptHistory(path)
		require.NoError(t, err)

		t.Run("it should group prompts by session id", func(t *testing.T) {
			assert.Equal(t, []int{2, 1}, []int{len(history["a"]), len(history["b"])})
		})

		t.Run("it should keep each session's prompts in the order typed", func(t *testing.T) {
			assert.Equal(t, []string{"look at PROJ-1", "[Pasted text #1]"}, []string{history["a"][0].Text, history["a"][1].Text})
		})

		t.Run("it should carry each prompt's time", func(t *testing.T) {
			assert.Equal(t, time.UnixMilli(1792131816410), history["b"][0].At)
		})

		t.Run("it should carry each prompt's directory", func(t *testing.T) {
			assert.Equal(t, "/w/app", history["b"][0].Dir)
		})

		t.Run("it should carry what was pasted", func(t *testing.T) {
			assert.Equal(t, []string{"the plan"}, history["a"][1].Pasted)
		})

		t.Run("and a line is malformed or the last line is partial", func(t *testing.T) {
			path := write(t, `{"display":"one","timestamp":1,"project":"/w","sessionId":"a"}
not json
{"display":"two","timestamp":2,"project":"/w","sessionId":"a"}
{"display":"thr`)
			history, err := claude.ReadPromptHistory(path)
			require.NoError(t, err)

			t.Run("it should skip it and keep the rest", func(t *testing.T) {
				assert.Len(t, history["a"], 2)
			})
		})
	})

	t.Run("when a line begins with NUL bytes", func(t *testing.T) {
		path := write(t, "\x00\x00\x00{\"display\":\"lost\",\"timestamp\":1,\"project\":\"/w\",\"sessionId\":\"lost\"}garbage\n"+
			`{"display":"kept","timestamp":2,"project":"/w","sessionId":"kept"}`+"\n")
		history, err := claude.ReadPromptHistory(path)
		require.NoError(t, err)

		t.Run("it should skip it and keep reading", func(t *testing.T) {
			assert.Equal(t, []int{1, 0}, []int{len(history["kept"]), len(history["lost"])})
		})
	})

	t.Run("when the file does not exist", func(t *testing.T) {
		history, err := claude.ReadPromptHistory(filepath.Join(t.TempDir(), "missing.jsonl"))

		t.Run("it should return no error", func(t *testing.T) {
			assert.NoError(t, err)
		})

		t.Run("it should return no prompts", func(t *testing.T) {
			assert.Empty(t, history)
		})
	})
}
