package config_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
)

// known stands in for the city's key names.
func known(name string) bool {
	switch strings.ToLower(name) {
	case "k", "m", "y", "h", "q", "w", "s", "n", "f3":
		return true
	}
	return false
}

func TestParseChord(t *testing.T) {
	t.Run("when a chord names modifiers and a key", func(t *testing.T) {
		c, err := config.ParseChord(config.KeyFindShow, "ctrl+alt+m", "linux", known)
		require.NoError(t, err)

		t.Run("it should read both", func(t *testing.T) {
			assert.Equal(t, config.Chord{Ctrl: true, Alt: true, Key: "m"}, c)
		})
	})

	for _, tc := range []struct {
		goos string
		want config.Chord
	}{
		{"darwin", config.Chord{Cmd: true, Key: "k"}},
		{"linux", config.Chord{Ctrl: true, Key: "k"}},
	} {
		t.Run("when it uses mod on "+tc.goos, func(t *testing.T) {
			c, err := config.ParseChord(config.KeyFind, "mod+k", tc.goos, known)
			require.NoError(t, err)

			t.Run("it should be ⌘ on macOS and Ctrl elsewhere", func(t *testing.T) {
				assert.Equal(t, tc.want, c)
			})
		})
	}

	t.Run("when it names a key Ebitengine does not know", func(t *testing.T) {
		_, err := config.ParseChord(config.KeyFindShow, "ctrl+banana", "linux", known)

		t.Run("it should refuse it, naming the setting", func(t *testing.T) {
			assert.ErrorContains(t, err, config.KeyFindShow)
		})
	})

	for key, does := range map[string]string{"m": "minimizes", "h": "hides", "q": "quits", "w": "closes"} {
		t.Run("when it is ⌘"+strings.ToUpper(key)+", which macOS keeps, on macOS", func(t *testing.T) {
			_, err := config.ParseChord(config.KeyFindShow, "cmd+"+key, "darwin", known)

			t.Run("it should refuse it and say what macOS does with it", func(t *testing.T) {
				assert.ErrorContains(t, err, does)
			})
		})
	}

	for _, tc := range []struct {
		goos                  string
		open, copy, show, new config.Chord
	}{
		{"darwin", config.Chord{Cmd: true, Key: "k"}, config.Chord{Cmd: true, Key: "y"}, config.Chord{Ctrl: true, Key: "m"}, config.Chord{Shift: true, Key: "n"}},
		{"linux", config.Chord{Ctrl: true, Key: "k"}, config.Chord{Ctrl: true, Key: "y"}, config.Chord{Ctrl: true, Key: "m"}, config.Chord{Shift: true, Key: "n"}},
	} {
		t.Run("when nothing is set, on "+tc.goos, func(t *testing.T) {
			chords, err := config.FindChords(config.Config{}, tc.goos, known)
			require.NoError(t, err)

			t.Run("it should open the finder on mod+k, copy on mod+y, show on ctrl+m, and open New session on shift+n", func(t *testing.T) {
				assert.Equal(t, config.Chords{Open: tc.open, Copy: tc.copy, Show: tc.show, New: tc.new}, chords)
			})
		})
	}
}
