package claude_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"botropolis.city/botropolis/pkg/claude"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func said(role, ts, text string) string {
	return fmt.Sprintf(`{"type":%q,"isSidechain":false,"timestamp":%q,"sessionId":%q,`+
		`"message":{"role":%q,"content":[{"type":"text","text":%q}]}}`, role, ts, sid, role, text)
}

func typed(ts, text string) string {
	return fmt.Sprintf(`{"type":"user","isSidechain":false,"timestamp":%q,"sessionId":%q,`+
		`"message":{"role":"user","content":%q}}`, ts, sid, text)
}

func TestExcerpt(t *testing.T) {
	const ts = "2026-10-06T10:00:00.000Z"

	t.Run("when the transcript holds a conversation with tools and asides", func(t *testing.T) {
		path := writeTranscript(t, userLine,
			typed(ts, "add a summary button"),
			toolCall("msg_01", ts, "toolu_01", "Bash"),
			toolResult(ts, "toolu_01"),
			said("assistant", ts, "the recap is in the transcript"),
			sidechain(said("assistant", ts, "a subagent's aside")),
			metaPrompt(ts),
			typed(ts, "<task-notification>done</task-notification>"),
			typed(ts, "ship it"))
		excerpt, err := claude.Excerpt(path, 10_000)
		require.NoError(t, err)

		t.Run("it should keep the prompts and replies in order, and nothing else", func(t *testing.T) {
			assert.Equal(t, "User: add a summary button\n\nClaude: the recap is in the transcript\n\nUser: ship it", excerpt)
		})
	})

	t.Run("when the conversation is longer than the limit", func(t *testing.T) {
		path := writeTranscript(t, userLine,
			typed(ts, strings.Repeat("old ", 50)),
			typed(ts, "newest"))
		excerpt, err := claude.Excerpt(path, 20)

		t.Run("it should keep the newest turns that fit", func(t *testing.T) {
			require.NoError(t, err)
			assert.Equal(t, "User: newest", excerpt)
		})
	})

	t.Run("when the transcript does not exist", func(t *testing.T) {
		_, err := claude.Excerpt(filepath.Join(t.TempDir(), "missing.jsonl"), 100)

		t.Run("it should say so", func(t *testing.T) {
			assert.Error(t, err)
		})
	})
}

func fakeClaude(t *testing.T, script string) (bin, out string) {
	t.Helper()
	dir := t.TempDir()
	bin = filepath.Join(dir, "claude")
	out = filepath.Join(dir, "out")
	body := "#!/usr/bin/env bash\nOUT=" + out + "\n" + script + "\n"
	require.NoError(t, os.WriteFile(bin, []byte(body), 0o755))
	return bin, out
}

func TestSummarise(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	t.Run("when claude answers", func(t *testing.T) {
		bin, out := fakeClaude(t, `printf '%s\n' "$@" > "$OUT.args"; pwd > "$OUT.pwd"; cat > "$OUT.stdin"; printf '  the summary\n\n'`)
		summary, err := claude.Summarise(context.Background(), bin, "User: hello")
		require.NoError(t, err)
		read := func(name string) string {
			data, err := os.ReadFile(out + "." + name)
			require.NoError(t, err)
			return string(data)
		}

		t.Run("it should return the answer trimmed", func(t *testing.T) {
			assert.Equal(t, "the summary", summary)
		})

		t.Run("it should run inside the fenced summary directory", func(t *testing.T) {
			assert.Equal(t, claude.SummaryDir()+"\n", read("pwd"))
		})

		t.Run("it should run print mode without persistence, settings, MCP servers or tools", func(t *testing.T) {
			assert.Equal(t, "-p\n--no-session-persistence\n--setting-sources\n\n--strict-mcp-config\n--tools\n\n--model\nhaiku\n", read("args"))
		})

		t.Run("it should pass the excerpt on stdin rather than the command line", func(t *testing.T) {
			assert.Contains(t, read("stdin"), "User: hello")
		})
	})

	t.Run("when claude leaves an empty project folder behind", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		folder := filepath.Join(home, ".claude", "projects", claude.ProjectFolder(claude.SummaryDir()))
		bin, _ := fakeClaude(t, `mkdir -p "`+folder+`/memory"; echo summary`)
		_, err := claude.Summarise(context.Background(), bin, "User: hello")
		require.NoError(t, err)

		t.Run("it should remove it", func(t *testing.T) {
			assert.NoDirExists(t, folder)
		})
	})

	t.Run("when the project folder holds something", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		folder := filepath.Join(home, ".claude", "projects", claude.ProjectFolder(claude.SummaryDir()))
		kept := filepath.Join(folder, "memory", "MEMORY.md")
		bin, _ := fakeClaude(t, `mkdir -p "`+folder+`/memory"; touch "`+kept+`"; echo summary`)
		_, err := claude.Summarise(context.Background(), bin, "User: hello")
		require.NoError(t, err)

		t.Run("it should leave it alone", func(t *testing.T) {
			assert.FileExists(t, kept)
		})
	})

	t.Run("when claude fails", func(t *testing.T) {
		bin, _ := fakeClaude(t, `echo "not logged in" >&2; exit 1`)
		_, err := claude.Summarise(context.Background(), bin, "User: hello")

		t.Run("it should report what claude said", func(t *testing.T) {
			assert.ErrorContains(t, err, "not logged in")
		})
	})

	t.Run("when claude fails and explains on stdout", func(t *testing.T) {
		bin, _ := fakeClaude(t, `cat > /dev/null; echo "Prompt is too long"; exit 1`)
		_, err := claude.Summarise(context.Background(), bin, "User: hello")

		t.Run("it should report what claude said", func(t *testing.T) {
			assert.ErrorContains(t, err, "Prompt is too long")
		})
	})

	t.Run("when claude answers with nothing", func(t *testing.T) {
		bin, _ := fakeClaude(t, `cat > /dev/null`)
		_, err := claude.Summarise(context.Background(), bin, "User: hello")

		t.Run("it should be an error, not an empty summary", func(t *testing.T) {
			assert.Error(t, err)
		})
	})
}
