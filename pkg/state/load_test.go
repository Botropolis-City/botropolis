package state_test

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	liveRecord = `{"pid":%d,"sessionId":"` + sidA + `","cwd":"/home/avesta/workspaces/github/mCedar/cinders",` +
		`"startedAt":1789617482873,"version":"2.1.273","kind":"interactive","entrypoint":"cli","name":"record name",` +
		`"status":"idle","updatedAt":1789617584013}`
	transcriptLines = `{"type":"user","isSidechain":false,"timestamp":"2026-09-16T19:50:32.231Z","sessionId":"` + sidA + `",` +
		`"cwd":"/home/avesta/workspaces/github/mCedar/cinders","gitBranch":"main","version":"2.1.273","entrypoint":"cli",` +
		`"message":{"role":"user","content":"hi"}}
{"type":"assistant","isSidechain":false,"timestamp":"2026-09-16T19:50:40.000Z","sessionId":"` + sidA + `",` +
		`"message":{"id":"msg_01","model":"claude-opus-5[1m]","role":"assistant","stop_reason":"end_turn",` +
		`"content":[{"type":"text","text":"hello"}],"usage":{"input_tokens":2,"output_tokens":5,"cache_read_input_tokens":100000}}}
{"type":"ai-title","aiTitle":"Fix the CI queue","sessionId":"` + sidA + `"}
`
	bookkeepingTail = `{"type":"system","subtype":"compact_boundary","isSidechain":false,"timestamp":"2026-09-16T20:01:32.136Z","sessionId":"` + sidA + `","content":"Conversation compacted"}
{"type":"user","isSidechain":false,"isMeta":true,"timestamp":"2026-09-16T20:01:32.112Z","sessionId":"` + sidA + `","message":{"role":"user","content":"<summary>"}}
{"type":"user","isSidechain":false,"timestamp":"2026-09-16T20:01:32.234Z","sessionId":"` + sidA + `","message":{"role":"user","content":"carry on"}}
{"type":"attachment","sessionId":"` + sidA + `","timestamp":"2026-09-16T20:01:32.240Z","attachment":{"type":"skill_listing"}}
{"type":"last-prompt","sessionId":"` + sidA + `","lastPrompt":"carry on"}
{"type":"agent-name","sessionId":"` + sidA + `","agentName":"record name"}
{"type":"mode","sessionId":"` + sidA + `","mode":"default"}
{"type":"permission-mode","sessionId":"` + sidA + `","permissionMode":"default"}
{"type":"atis-latch","sessionId":"` + sidA + `","atis":true}
{"type":"worktree-state","sessionId":"` + sidA + `","worktreeSession":null}
`
)

func writeHome(t *testing.T, pid int) string {
	t.Helper()
	home := t.TempDir()
	sessions := filepath.Join(home, ".claude", "sessions")
	project := filepath.Join(home, ".claude", "projects", "-home-avesta-workspaces-github-mCedar-cinders")
	require.NoError(t, os.MkdirAll(sessions, 0o700))
	require.NoError(t, os.MkdirAll(project, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(sessions, strconv.Itoa(pid)+".json"),
		[]byte(fmt.Sprintf(liveRecord, pid)), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(project, sidA+".jsonl"), []byte(transcriptLines), 0o600))
	return home
}

func TestLoad(t *testing.T) {
	t.Run("when the home holds one live record with a transcript", func(t *testing.T) {
		snapshot, err := state.Load(writeHome(t, 4242), alive, now)
		require.NoError(t, err)
		require.Len(t, snapshot.Sessions, 1)
		session := snapshot.Sessions[0]

		t.Run("it should join the transcript to the record", func(t *testing.T) {
			assert.Equal(t, "Fix the CI queue", session.Title)
		})

		t.Run("it should derive the state", func(t *testing.T) {
			assert.Equal(t, state.NeedsYou, session.State)
		})

		t.Run("it should measure the context", func(t *testing.T) {
			assert.InDelta(t, 10.0002, session.ContextPercent, 1e-4)
		})

		t.Run("it should report nothing skipped", func(t *testing.T) {
			assert.Empty(t, snapshot.Skipped)
		})
	})

	t.Run("when an idle background record's transcript ends in fresh bookkeeping after a prompt with no hand-back", func(t *testing.T) {
		home := writeHome(t, 4242)
		rewrite(t, filepath.Join(home, ".claude", "sessions", "4242.json"),
			`"kind":"interactive"`, `"kind":"bg","jobId":"0898d7e4"`)
		appendFile(t, filepath.Join(home, ".claude", "projects", "-home-avesta-workspaces-github-mCedar-cinders", sidA+".jsonl"), bookkeepingTail)

		snapshot, err := state.Load(home, alive, now)
		require.NoError(t, err)
		require.Len(t, snapshot.Sessions, 1)

		t.Run("it should need you", func(t *testing.T) {
			assert.Equal(t, state.NeedsYou, snapshot.Sessions[0].State)
		})

		t.Run("it should keep the tail's turn for the card", func(t *testing.T) {
			assert.Equal(t, claude.TurnWorking, snapshot.Sessions[0].Turn)
		})
	})

	t.Run("when a background job's pty socket has a client connected", func(t *testing.T) {
		home := writeHome(t, 4242)
		sock := listenAndDial(t)
		require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude", "daemon"), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "daemon", "roster.json"),
			[]byte(`{"workers":{"0898d7e4":{"sessionId":"`+sidA+`","ptySock":"`+sock+`"}}}`), 0o600))
		rewrite(t, filepath.Join(home, ".claude", "sessions", "4242.json"),
			`"kind":"interactive"`, `"kind":"bg","jobId":"0898d7e4"`)
		rewrite(t, filepath.Join(home, ".claude", "sessions", "4242.json"), `"status":"idle"`, `"status":"busy"`)
		rewrite(t, filepath.Join(home, ".claude", "projects", "-home-avesta-workspaces-github-mCedar-cinders", sidA+".jsonl"),
			`"stop_reason":"end_turn"`, `"stop_reason":"tool_use"`)

		snapshot, err := state.Load(home, state.Probes{Alive: alive.Alive, Attached: state.UnixSocketConnected}, now)
		require.NoError(t, err)
		require.Len(t, snapshot.Sessions, 1)

		t.Run("it should see the session as attached", func(t *testing.T) {
			assert.True(t, snapshot.Sessions[0].Attached)
		})

		t.Run("it should be working rather than unattended", func(t *testing.T) {
			assert.Equal(t, state.Working, snapshot.Sessions[0].State)
		})
	})

	t.Run("when the home has no .claude directory", func(t *testing.T) {
		snapshot, err := state.Load(t.TempDir(), alive, now)

		t.Run("it should not return an error", func(t *testing.T) {
			assert.NoError(t, err)
		})

		t.Run("it should return no sessions", func(t *testing.T) {
			assert.Empty(t, snapshot.Sessions)
		})
	})
}

func TestUnixSocketConnected(t *testing.T) {
	t.Run("when a client is connected to the socket", func(t *testing.T) {
		t.Run("it should be connected", func(t *testing.T) {
			assert.True(t, state.UnixSocketConnected(listenAndDial(t)))
		})
	})

	t.Run("when the socket is listening with no client", func(t *testing.T) {
		sock := filepath.Join(shortTempDir(t), "pty.sock")
		listener, err := net.Listen("unix", sock)
		require.NoError(t, err)
		t.Cleanup(func() { _ = listener.Close() })

		t.Run("it should not be connected", func(t *testing.T) {
			assert.False(t, state.UnixSocketConnected(sock))
		})
	})

	t.Run("when the socket does not exist", func(t *testing.T) {
		t.Run("it should not be connected", func(t *testing.T) {
			assert.False(t, state.UnixSocketConnected(filepath.Join(t.TempDir(), "missing.sock")))
		})
	})
}

func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "bt")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func listenAndDial(t *testing.T) string {
	t.Helper()
	sock := filepath.Join(shortTempDir(t), "pty.sock")
	listener, err := net.Listen("unix", sock)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	client, err := net.Dial("unix", sock)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return sock
}

func appendFile(t *testing.T, path, lines string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	_, err = f.WriteString(lines)
	require.NoError(t, err)
	require.NoError(t, f.Close())
}

func rewrite(t *testing.T, path, old, replacement string) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(data), old)
	require.NoError(t, os.WriteFile(path, []byte(strings.Replace(string(data), old, replacement, 1)), 0o600))
}

func TestProcessAlive(t *testing.T) {
	t.Run("when asked about this test's own process", func(t *testing.T) {
		t.Run("it should be alive", func(t *testing.T) {
			assert.True(t, state.ProcessAlive(os.Getpid()))
		})
	})

	t.Run("when asked about a pid beyond the kernel's range", func(t *testing.T) {
		t.Run("it should not be alive", func(t *testing.T) {
			assert.False(t, state.ProcessAlive(1<<30))
		})
	})
}
