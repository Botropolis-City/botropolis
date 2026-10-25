package hotkey

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
)

func TestCarbonChord(t *testing.T) {
	for _, c := range []struct {
		chord config.Chord
		mods  uint32
		code  uint32
		names string
	}{
		{config.Chord{Ctrl: true, Alt: true, Key: "f"}, 0x1000 | 0x0800, 0x03, "controlKey and optionKey"},
		{config.Chord{Cmd: true, Shift: true, Key: "f1"}, 0x0100 | 0x0200, 0x7a, "cmdKey and shiftKey"},
	} {
		t.Run("when "+c.chord.String()+" becomes a Carbon hot key", func(t *testing.T) {
			k, err := carbonChord(c.chord)
			require.NoError(t, err)

			t.Run("it should carry "+c.names, func(t *testing.T) {
				assert.Equal(t, c.mods, k.mods)
			})

			t.Run("it should use its key's virtual key code", func(t *testing.T) {
				assert.Equal(t, c.code, k.code)
			})
		})
	}

	t.Run("when the key has no Carbon key code", func(t *testing.T) {
		_, err := carbonChord(config.Chord{Ctrl: true, Key: "printscreen"})

		t.Run("it should say so", func(t *testing.T) {
			assert.ErrorContains(t, err, "not a key a global shortcut can use on macOS")
		})
	})
}
