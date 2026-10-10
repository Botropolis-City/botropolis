package app_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"botropolis.city/botropolis/pkg/app"
)

func TestModule(t *testing.T) {
	t.Run("when the dependency graph is validated", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())

		t.Run("it should resolve every provider", func(t *testing.T) {
			assert.NoError(t, fx.ValidateApp(app.Module, fx.WithLogger(func() fxevent.Logger { return fxevent.NopLogger })))
		})
	})
}

func TestRun(t *testing.T) {
	t.Run("when run with --version", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())

		t.Run("it should exit zero", func(t *testing.T) {
			assert.Equal(t, 0, app.Run(context.Background(), []string{"--version"}))
		})
	})

	t.Run("when run with status against an empty home", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		stdout := captureStdout(t)

		t.Run("it should exit zero", func(t *testing.T) {
			assert.Equal(t, 0, app.Run(context.Background(), []string{"status", "--home", t.TempDir()}))
		})

		t.Run("it should scan that home rather than ask a daemon", func(t *testing.T) {
			assert.Equal(t, "no sessions\n", stdout())
		})
	})

	t.Run("when run with an unknown command", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())

		t.Run("it should exit one", func(t *testing.T) {
			assert.Equal(t, 1, app.Run(context.Background(), []string{"dance"}))
		})
	})
}

func TestRunExitCodes(t *testing.T) {
	for _, refusal := range []struct {
		name   string
		claude string
		code   int
	}{
		{"when claude refuses a directory nobody has trusted", "Workspace not trusted. Run `claude` in this directory once and accept the trust prompt, then retry.", 3},
		{"when claude fails for any other reason", "no such directory", 1},
	} {
		t.Run(refusal.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			fakeClaudeFailing(t, refusal.claude)

			t.Run(fmt.Sprintf("it should exit %d from new", refusal.code), func(t *testing.T) {
				assert.Equal(t, refusal.code, app.Run(context.Background(), []string{"new", t.TempDir()}))
			})
		})
	}
}

func fakeClaudeFailing(t *testing.T, message string) {
	t.Helper()
	bin := t.TempDir()
	script := "#!/bin/sh\nprintf '%s\\n' \"$BOTROPOLIS_FAKE_CLAUDE\" >&2\nexit 1\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0o755))
	t.Setenv("BOTROPOLIS_FAKE_CLAUDE", message)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func captureStdout(t *testing.T) func() string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	original := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = original })
	return func() string {
		_ = w.Close()
		os.Stdout = original
		data, _ := io.ReadAll(r)
		return string(data)
	}
}
