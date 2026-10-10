package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/testing/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sid = "08b1d655-276f-495b-a61f-2214af35a97a"
	cwd = "/home/avesta/workspaces/github/mCedar/cinders"
)

func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "bt")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func startDaemon(t *testing.T, home, sock string) (*bytes.Buffer, <-chan int) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	var out bytes.Buffer
	exit := make(chan int, 1)
	go func() { exit <- run(ctx, []string{"--home", home, "--socket", sock}, &out) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-exit:
		case <-time.After(5 * time.Second):
			t.Error("daemon did not stop")
		}
	})
	require.Eventually(t, func() bool {
		client, err := proto.Dial(sock)
		if err != nil {
			return false
		}
		_ = client.Close()
		return true
	}, 3*time.Second, 20*time.Millisecond)
	return &out, exit
}

func TestRun(t *testing.T) {
	t.Run("when invoked with --version", func(t *testing.T) {
		var out bytes.Buffer
		code := run(context.Background(), []string{"--version"}, &out)

		t.Run("it should exit zero", func(t *testing.T) {
			assert.Equal(t, 0, code)
		})

		t.Run("it should print the binary name and version", func(t *testing.T) {
			assert.Equal(t, "botropolisd dev\n", out.String())
		})
	})

	t.Run("when invoked with an unknown flag", func(t *testing.T) {
		var out bytes.Buffer
		code := run(context.Background(), []string{"--bogus"}, &out)

		t.Run("it should exit non-zero", func(t *testing.T) {
			assert.NotEqual(t, 0, code)
		})
	})

	t.Run("when started over a home with one session", func(t *testing.T) {
		home := helpers.NewHome(t).Session(os.Getpid(), sid, cwd, "interactive", "idle")
		home.Transcript(sid, cwd,
			helpers.UserPrompt(sid, cwd, "2026-09-16T19:50:00.000Z"),
			helpers.AssistantReply(sid, "msg_01", "2026-09-16T19:50:10.000Z"),
			helpers.AITitle(sid, "Fix the CI queue"))
		sock := filepath.Join(shortTempDir(t), "d", "botropolis.sock")
		startDaemon(t, home.Path, sock)

		t.Run("it should serve a snapshot over the socket", func(t *testing.T) {
			client, err := proto.Dial(sock)
			require.NoError(t, err)
			defer func() { _ = client.Close() }()
			snapshot, err := client.Snapshot()
			require.NoError(t, err)
			require.Len(t, snapshot.Sessions, 1)
			assert.Equal(t, "Fix the CI queue", snapshot.Sessions[0].Title)
		})

		t.Run("it should create the socket directory privately", func(t *testing.T) {
			info, err := os.Stat(filepath.Dir(sock))
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
		})

		t.Run("and a transcript changes on disk", func(t *testing.T) {
			home.AppendTranscript(sid, cwd, helpers.UserPrompt(sid, cwd, "2026-09-16T19:51:00.000Z"))

			t.Run("it should pick it up", func(t *testing.T) {
				assert.Eventually(t, func() bool {
					client, err := proto.Dial(sock)
					if err != nil {
						return false
					}
					defer func() { _ = client.Close() }()
					snapshot, err := client.Snapshot()
					return err == nil && len(snapshot.Sessions) == 1 && snapshot.Sessions[0].Turn == "working"
				}, 3*time.Second, 20*time.Millisecond)
			})
		})
	})

	t.Run("when a stale socket file is left behind", func(t *testing.T) {
		home := helpers.NewHome(t)
		sock := filepath.Join(shortTempDir(t), "botropolis.sock")
		require.NoError(t, os.WriteFile(sock, nil, 0o600))
		startDaemon(t, home.Path, sock)

		t.Run("it should replace it and serve", func(t *testing.T) {
			client, err := proto.Dial(sock)
			require.NoError(t, err)
			defer func() { _ = client.Close() }()
			_, err = client.Snapshot()
			assert.NoError(t, err)
		})
	})

	t.Run("when the socket path is too long for a unix socket", func(t *testing.T) {
		var out bytes.Buffer
		sock := filepath.Join(t.TempDir(), strings.Repeat("x", 120), "botropolis.sock")
		code := run(context.Background(), []string{"--home", t.TempDir(), "--socket", sock}, &out)

		t.Run("it should exit non-zero", func(t *testing.T) {
			assert.Equal(t, 1, code)
		})

		t.Run("it should say why", func(t *testing.T) {
			assert.Contains(t, out.String(), "at most 107")
		})
	})

	t.Run("when the context is cancelled before the daemon has started", func(t *testing.T) {
		home := helpers.NewHome(t)
		sock := filepath.Join(shortTempDir(t), "botropolis.sock")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		var out bytes.Buffer
		code := run(ctx, []string{"--home", home.Path, "--socket", sock}, &out)

		t.Run("it should exit zero", func(t *testing.T) {
			assert.Equal(t, 0, code, out.String())
		})

		t.Run("it should remove its socket", func(t *testing.T) {
			_, err := os.Stat(sock)
			assert.True(t, os.IsNotExist(err))
		})
	})

	t.Run("when the context is cancelled", func(t *testing.T) {
		home := helpers.NewHome(t)
		sock := filepath.Join(shortTempDir(t), "botropolis.sock")
		ctx, cancel := context.WithCancel(context.Background())
		var out bytes.Buffer
		exit := make(chan int, 1)
		go func() { exit <- run(ctx, []string{"--home", home.Path, "--socket", sock}, &out) }()
		require.Eventually(t, func() bool { _, err := os.Stat(sock); return err == nil }, 3*time.Second, 20*time.Millisecond)
		cancel()

		t.Run("it should exit zero", func(t *testing.T) {
			select {
			case code := <-exit:
				assert.Equal(t, 0, code)
			case <-time.After(5 * time.Second):
				t.Fatal("daemon did not stop")
			}
		})

		t.Run("it should remove its socket", func(t *testing.T) {
			_, err := os.Stat(sock)
			assert.True(t, os.IsNotExist(err))
		})
	})
}
