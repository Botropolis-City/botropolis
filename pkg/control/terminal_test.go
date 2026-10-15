package control_test

import (
	"testing"

	"botropolis.city/botropolis/pkg/control"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func onPath(names ...string) func(string) bool {
	set := map[string]bool{}
	for _, n := range names {
		set[n] = true
	}
	return func(name string) bool { return set[name] }
}

var attach = []string{"claude", "attach", "0898d7e4"}

var linux = control.DesktopFor("linux")

func TestTerminalCommand(t *testing.T) {
	t.Run("when BOTROPOLIS_TERMINAL is set", func(t *testing.T) {
		argv, err := linux.TerminalCommand(env(map[string]string{"BOTROPOLIS_TERMINAL": "wezterm start --", "TERMINAL": "kitty"}), onPath(), attach)
		require.NoError(t, err)

		t.Run("it should use it verbatim and append the command", func(t *testing.T) {
			assert.Equal(t, []string{"wezterm", "start", "--", "claude", "attach", "0898d7e4"}, argv)
		})
	})

	t.Run("when BOTROPOLIS_TERMINAL is only the name of a known terminal, as the settings picker stores it", func(t *testing.T) {
		argv, err := linux.TerminalCommand(env(map[string]string{"BOTROPOLIS_TERMINAL": "alacritty"}), onPath(), attach)
		require.NoError(t, err)

		t.Run("it should use that terminal's own convention", func(t *testing.T) {
			assert.Equal(t, []string{"alacritty", "-e", "claude", "attach", "0898d7e4"}, argv)
		})
	})

	t.Run("when only TERMINAL is set", func(t *testing.T) {
		cases := []struct {
			terminal string
			want     []string
		}{
			{"kitty", []string{"kitty", "claude", "attach", "0898d7e4"}},
			{"foot", []string{"foot", "claude", "attach", "0898d7e4"}},
			{"alacritty", []string{"alacritty", "-e", "claude", "attach", "0898d7e4"}},
			{"wezterm", []string{"wezterm", "start", "--", "claude", "attach", "0898d7e4"}},
			{"gnome-terminal", []string{"gnome-terminal", "--", "claude", "attach", "0898d7e4"}},
			{"konsole", []string{"konsole", "-e", "claude", "attach", "0898d7e4"}},
			{"xterm", []string{"xterm", "-e", "claude", "attach", "0898d7e4"}},
			{"terminator", []string{"terminator", "-x", "claude", "attach", "0898d7e4"}},
			{"/usr/bin/st", []string{"/usr/bin/st", "-e", "claude", "attach", "0898d7e4"}},
		}
		for _, c := range cases {
			t.Run("and it is "+c.terminal, func(t *testing.T) {
				argv, err := linux.TerminalCommand(env(map[string]string{"TERMINAL": c.terminal}), onPath(), attach)
				require.NoError(t, err)

				t.Run("it should use that terminal's own convention", func(t *testing.T) {
					assert.Equal(t, c.want, argv)
				})
			})
		}
	})

	t.Run("when neither is set", func(t *testing.T) {
		t.Run("and foot is on the path", func(t *testing.T) {
			argv, err := linux.TerminalCommand(env(nil), onPath("foot", "xterm"), attach)
			require.NoError(t, err)

			t.Run("it should pick the first known terminal found", func(t *testing.T) {
				assert.Equal(t, []string{"foot", "claude", "attach", "0898d7e4"}, argv)
			})
		})

		t.Run("and no known terminal is on the path", func(t *testing.T) {
			_, err := linux.TerminalCommand(env(nil), onPath(), attach)

			t.Run("it should return an error naming the variables to set", func(t *testing.T) {
				assert.ErrorContains(t, err, "BOTROPOLIS_TERMINAL")
			})
		})
	})
}
