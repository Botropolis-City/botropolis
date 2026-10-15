package control_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/control"
)

var appleTerminal = []string{"osascript",
	"-e", `tell application "Terminal"`,
	"-e", "activate",
	"-e", `do script "claude attach 0898d7e4"`,
	"-e", "end tell"}

func TestDesktop(t *testing.T) {
	cases := []struct {
		goos      string
		opener    []string
		clipboard string
		feeds     []string
	}{
		{"linux", []string{"xdg-open", dir}, "xclip", []string{"xclip", "-selection", "clipboard"}},
		{"darwin", []string{"open", dir}, "pbcopy", []string{"pbcopy"}},
	}
	for _, c := range cases {
		t.Run("when the desktop is "+c.goos, func(t *testing.T) {
			desktop := control.DesktopFor(c.goos)

			t.Run("and a directory is revealed", func(t *testing.T) {
				runner := &fakeRunner{}
				require.NoError(t, control.New(runner, desktop, env(nil), onPath()).Reveal(dir))

				t.Run("it should open it with the desktop's opener", func(t *testing.T) {
					require.Len(t, runner.starts, 1)
					assert.Equal(t, c.opener, runner.starts[0].argv)
				})
			})

			t.Run("and a path is copied with "+c.clipboard+" on the path", func(t *testing.T) {
				runner := &fakeRunner{}
				require.NoError(t, control.New(runner, desktop, env(nil), onPath(c.clipboard)).Copy(context.Background(), dir))

				t.Run("it should feed the path to "+c.clipboard, func(t *testing.T) {
					require.Len(t, runner.feeds, 1)
					assert.Equal(t, c.feeds, runner.feeds[0].argv)
				})
			})
		})
	}

	t.Run("when the desktop is darwin", func(t *testing.T) {
		mac := control.DesktopFor("darwin")

		t.Run("and no terminal is named and none is on the path", func(t *testing.T) {
			argv, err := mac.TerminalCommand(env(nil), onPath("osascript"), attach)
			require.NoError(t, err)

			t.Run("it should open Terminal through osascript", func(t *testing.T) {
				assert.Equal(t, appleTerminal, argv)
			})
		})

		t.Run("and TERMINAL names Terminal", func(t *testing.T) {
			argv, err := mac.TerminalCommand(env(map[string]string{"TERMINAL": "Terminal"}), onPath("osascript"), attach)
			require.NoError(t, err)

			t.Run("it should open Terminal through osascript", func(t *testing.T) {
				assert.Equal(t, appleTerminal, argv)
			})
		})

		t.Run("and wezterm is on the path", func(t *testing.T) {
			argv, err := mac.TerminalCommand(env(nil), onPath("wezterm", "osascript"), attach)
			require.NoError(t, err)

			t.Run("it should prefer it to Terminal", func(t *testing.T) {
				assert.Equal(t, []string{"wezterm", "start", "--", "claude", "attach", "0898d7e4"}, argv)
			})
		})

		t.Run("and a command has quotes and spaces", func(t *testing.T) {
			argv, err := mac.TerminalCommand(env(nil), onPath("osascript"), []string{"echo", `it's "here"`})
			require.NoError(t, err)

			t.Run("it should quote it for the shell and then for AppleScript", func(t *testing.T) {
				assert.Equal(t, `do script "echo 'it'\\''s \"here\"'"`, argv[6])
			})
		})

		t.Run("it should offer Terminal among its known terminals", func(t *testing.T) {
			assert.Contains(t, mac.KnownTerminals(), "Terminal")
		})
	})

	t.Run("when the desktop is linux", func(t *testing.T) {
		t.Run("it should not offer Terminal", func(t *testing.T) {
			assert.NotContains(t, control.DesktopFor("linux").KnownTerminals(), "Terminal")
		})
	})
}
