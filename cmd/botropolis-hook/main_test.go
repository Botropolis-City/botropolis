package main

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/daemon"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sid = "08b1d655-276f-495b-a61f-2214af35a97a"
	cwd = "/home/avesta/workspaces/github/mCedar/cinders"
)

var probes = state.Probes{Alive: func(int) bool { return true }, Attached: func(string) bool { return false }}

func serveDaemon(t *testing.T) (*daemon.Daemon, string) {
	t.Helper()
	home := helpers.NewHome(t).Session(4242, sid, cwd, "interactive", "idle")
	home.Transcript(sid, cwd,
		helpers.UserPrompt(sid, cwd, "2026-09-16T19:50:00.000Z"),
		helpers.AssistantReply(sid, "msg_01", "2026-09-16T19:50:10.000Z"))
	d := daemon.New(home.Path, probes, time.Now)
	require.NoError(t, d.Rescan())
	dir, err := os.MkdirTemp("", "bt")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	sock := filepath.Join(dir, "botropolis.sock")
	listener, err := net.Listen("unix", sock)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = d.Serve(ctx, listener)
	}()
	t.Cleanup(func() {
		cancel()
		_ = listener.Close()
		<-done
	})
	return d, sock
}

func TestRun(t *testing.T) {
	t.Run("when invoked with --version", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := run([]string{"--version"}, strings.NewReader(""), &out, &errOut)

		t.Run("it should exit zero", func(t *testing.T) {
			assert.Equal(t, 0, code)
		})

		t.Run("it should print the binary name and version", func(t *testing.T) {
			assert.Equal(t, "botropolis-hook dev\n", out.String())
		})
	})

	t.Run("when a PreToolUse payload arrives on stdin and the daemon is up", func(t *testing.T) {
		d, sock := serveDaemon(t)
		payload := `{"session_id":"` + sid + `","hook_event_name":"PreToolUse","tool_name":"Bash","cwd":"` + cwd + `"}`
		var out, errOut bytes.Buffer
		code := run([]string{"--socket", sock}, strings.NewReader(payload), &out, &errOut)

		t.Run("it should exit zero", func(t *testing.T) {
			assert.Equal(t, 0, code)
		})

		t.Run("it should print nothing to stdout", func(t *testing.T) {
			assert.Empty(t, out.String())
		})

		t.Run("it should reach the daemon", func(t *testing.T) {
			assert.Equal(t, "Bash", d.Snapshot().Sessions[0].Tool)
		})
	})

	t.Run("when the daemon is not running", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := run([]string{"--socket", filepath.Join(t.TempDir(), "missing.sock")},
			strings.NewReader(`{"session_id":"`+sid+`","hook_event_name":"Stop"}`), &out, &errOut)

		t.Run("it should still exit zero", func(t *testing.T) {
			assert.Equal(t, 0, code)
		})

		t.Run("it should print nothing to stdout", func(t *testing.T) {
			assert.Empty(t, out.String())
		})

		t.Run("it should mention the failure on stderr", func(t *testing.T) {
			assert.Contains(t, errOut.String(), "missing.sock")
		})
	})

	t.Run("when stdin is empty", func(t *testing.T) {
		_, sock := serveDaemon(t)
		var out, errOut bytes.Buffer
		code := run([]string{"--socket", sock}, strings.NewReader(""), &out, &errOut)

		t.Run("it should still exit zero", func(t *testing.T) {
			assert.Equal(t, 0, code)
		})
	})
}
