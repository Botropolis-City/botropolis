package commands_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/control"
)

func doctorFor(t *testing.T, hooked bool, env map[string]string, onPath []string, dial error) commands.Doctor {
	t.Helper()
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0o700))
	if hooked {
		require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"hooks":{"Stop":[{"hooks":[{"command":"botropolis-hook"}]}]}}`), 0o600))
	}
	path := map[string]bool{}
	for _, p := range onPath {
		path[p] = true
	}
	return commands.Doctor{
		Home: home, Socket: "/run/none.sock", HookCommand: "botropolis-hook", CodexHome: filepath.Join(home, ".codex"),
		Desktop: control.DesktopFor("linux"),
		Getenv:  func(k string) string { return env[k] },
		OnPath:  func(n string) bool { return path[n] },
		Dial:    func(string) error { return dial },
	}
}

func checkNamed(checks []commands.Check, name string) commands.Check {
	for _, c := range checks {
		if c.Name == name {
			return c
		}
	}
	return commands.Check{}
}

func TestDoctor(t *testing.T) {
	t.Run("when everything is in place on X11", func(t *testing.T) {
		d := doctorFor(t, true, map[string]string{"DISPLAY": ":0", "TERMINAL": "kitty"}, []string{"claude", "kitty"}, nil)
		checks := d.Run()

		for _, name := range []string{"daemon", "hooks", "terminal", "claude", "harness claude", "display"} {
			t.Run("it should pass "+name, func(t *testing.T) {
				assert.Equal(t, commands.CheckOK, checkNamed(checks, name).Status, name)
			})
		}

		t.Run("it should say all good", func(t *testing.T) {
			var out bytes.Buffer
			require.NoError(t, d.Report(&out))
			assert.Contains(t, out.String(), "all good")
		})
	})

	t.Run("when the daemon is down and the hooks are missing", func(t *testing.T) {
		d := doctorFor(t, false, map[string]string{"DISPLAY": ":0"}, []string{"claude", "foot"}, errors.New("connection refused"))
		checks := d.Run()

		t.Run("it should warn about the daemon with the unit to start", func(t *testing.T) {
			c := checkNamed(checks, "daemon")
			assert.Equal(t, commands.CheckWarn, c.Status)
			assert.Contains(t, c.Hint, "botropolisd")
		})

		t.Run("it should warn about the hooks with the command to run", func(t *testing.T) {
			c := checkNamed(checks, "hooks")
			assert.Equal(t, commands.CheckWarn, c.Status)
			assert.Contains(t, c.Hint, "botropolis setup")
		})

		t.Run("it should print the hints", func(t *testing.T) {
			var out bytes.Buffer
			require.NoError(t, d.Report(&out))
			assert.Contains(t, out.String(), "botropolis setup")
			assert.NotContains(t, out.String(), "all good")
		})
	})

	t.Run("when there is no terminal and no claude", func(t *testing.T) {
		checks := doctorFor(t, true, map[string]string{"DISPLAY": ":0"}, nil, nil).Run()

		t.Run("it should fail the terminal check", func(t *testing.T) {
			assert.Equal(t, commands.CheckFail, checkNamed(checks, "terminal").Status)
		})

		t.Run("it should fail the claude check", func(t *testing.T) {
			assert.Equal(t, commands.CheckFail, checkNamed(checks, "claude").Status)
		})
	})

	t.Run("when the session is Wayland with XWayland", func(t *testing.T) {
		checks := doctorFor(t, true, map[string]string{"WAYLAND_DISPLAY": "wayland-1", "DISPLAY": ":0"}, []string{"claude", "foot"}, nil).Run()

		t.Run("it should pass with the XWayland note", func(t *testing.T) {
			c := checkNamed(checks, "display")
			assert.Equal(t, commands.CheckOK, c.Status)
			assert.Contains(t, c.Hint, "XWayland")
		})
	})

	t.Run("when there is no display", func(t *testing.T) {
		checks := doctorFor(t, true, nil, []string{"claude", "foot"}, nil).Run()

		t.Run("it should warn and name what still works", func(t *testing.T) {
			c := checkNamed(checks, "display")
			assert.Equal(t, commands.CheckWarn, c.Status)
			assert.Contains(t, c.Hint, "status")
		})
	})

	t.Run("when the desktop is macOS", func(t *testing.T) {
		t.Run("and the daemon is down, nothing sets DISPLAY and only osascript is on the path", func(t *testing.T) {
			d := doctorFor(t, true, nil, []string{"claude", "osascript"}, errors.New("connection refused"))
			d.Desktop = control.DesktopFor("darwin")
			checks := d.Run()

			t.Run("it should hint at the Homebrew service", func(t *testing.T) {
				assert.Contains(t, checkNamed(checks, "daemon").Hint, "brew services start botropolis")
			})

			t.Run("it should pass the display", func(t *testing.T) {
				assert.Equal(t, commands.CheckOK, checkNamed(checks, "display").Status)
			})

			t.Run("it should pass the terminal through Terminal", func(t *testing.T) {
				assert.Equal(t, commands.CheckOK, checkNamed(checks, "terminal").Status)
			})
		})
	})
}
