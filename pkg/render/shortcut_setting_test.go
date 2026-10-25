package render

import (
	"fmt"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/hotkey"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 84: the global shortcut's row records the next combination
// pressed, names one that means something elsewhere before saving it,
// and grabs nothing until one is set.
func TestShortcutSetting(t *testing.T) {
	// panel is the settings panel open on the global shortcut's row, set
	// to value, on a desktop that can grab.
	panel := func(value string) (*Game, *fakeShortcut, *[]ui.Setting) {
		var applied []ui.Setting
		s := &fakeShortcut{fired: make(chan uint32)}
		g := &Game{scene: city.NewScene(city.NewLayout()), scripted: -1, clicked: -1, shortcut: s, settingsOpen: true}
		g.settings = ui.NewSettings([]ui.Setting{{Key: config.KeyGlobalShortcut, Label: "global shortcut", Value: value}})
		g.apply = func(s ui.Setting) error {
			applied = append(applied, s)
			return nil
		}
		return g, s, &applied
	}
	// press is one tick with key just pressed and mods held.
	press := func(t *testing.T, g *Game, key ebiten.Key, mods ...string) {
		t.Helper()
		g.scripted = key
		for _, m := range mods {
			switch m {
			case "ctrl":
				g.scriptedCtrl = true
			case "alt":
				g.scriptedAlt = true
			case "shift":
				g.scriptedShift = true
			}
		}
		require.NoError(t, g.handleKeys())
		g.scripted, g.scriptedCtrl, g.scriptedAlt, g.scriptedShift = -1, false, false, false
	}
	saved := func(applied *[]ui.Setting) []string {
		var values []string
		for _, s := range *applied {
			values = append(values, s.Value)
		}
		return values
	}

	t.Run("when Enter is pressed on it", func(t *testing.T) {
		g, _, _ := panel("off")
		press(t, g, ebiten.KeyEnter)

		t.Run("it should ask for a combination", func(t *testing.T) {
			assert.Contains(t, g.settings.Note, "press a combination")
		})

		t.Run("and a combination is pressed", func(t *testing.T) {
			g, s, applied := panel("off")
			press(t, g, ebiten.KeyEnter)
			press(t, g, ebiten.KeyF, "ctrl", "alt")

			t.Run("it should record it", func(t *testing.T) {
				assert.Equal(t, []string{"ctrl+alt+f"}, saved(applied))
			})

			t.Run("it should grab it", func(t *testing.T) {
				require.NotNil(t, s.grabbed)
				assert.Equal(t, "ctrl+alt+f", s.grabbed.String())
			})
		})

		t.Run("and only a modifier is pressed", func(t *testing.T) {
			g, _, applied := panel("off")
			press(t, g, ebiten.KeyEnter)
			press(t, g, ebiten.KeyControlLeft, "ctrl")

			t.Run("it should keep waiting", func(t *testing.T) {
				assert.Equal(t, [2]any{true, 0}, [2]any{g.recordingShortcut, len(*applied)})
			})
		})

		t.Run("and Esc is pressed", func(t *testing.T) {
			g, _, applied := panel("ctrl+alt+f")
			press(t, g, ebiten.KeyEnter)
			press(t, g, ebiten.KeyEscape)

			t.Run("it should keep what was set", func(t *testing.T) {
				assert.Equal(t, [3]any{"ctrl+alt+f", 0, true}, [3]any{g.settings.Items[0].Value, len(*applied), g.settingsOpen})
			})
		})

		t.Run("and the combination commonly means something elsewhere", func(t *testing.T) {
			g, _, applied := panel("off")
			press(t, g, ebiten.KeyEnter)
			press(t, g, ebiten.KeyK, "ctrl")

			t.Run("it should name what", func(t *testing.T) {
				assert.Contains(t, g.settings.Note, hotkey.CommonMeaning(config.Chord{Ctrl: true, Key: "k"}))
			})

			t.Run("it should not save it yet", func(t *testing.T) {
				assert.Empty(t, *applied)
			})

			t.Run("and Enter is pressed", func(t *testing.T) {
				press(t, g, ebiten.KeyEnter)

				t.Run("it should save it anyway", func(t *testing.T) {
					assert.Equal(t, []string{"ctrl+k"}, saved(applied))
				})
			})
		})
	})

	for _, key := range []ebiten.Key{ebiten.KeyBackspace, ebiten.KeyDelete} {
		t.Run(fmt.Sprintf("when %s is pressed on a set row", key), func(t *testing.T) {
			g, s, applied := panel("ctrl+alt+f")
			s.grabbed = &config.Chord{Ctrl: true, Alt: true, Key: "f"}
			press(t, g, key)

			t.Run("it should clear it to off", func(t *testing.T) {
				assert.Equal(t, []string{"off"}, saved(applied))
			})

			t.Run("it should release the grab at once", func(t *testing.T) {
				assert.Equal(t, [2]any{1, true}, [2]any{s.released, s.grabbed == nil})
			})
		})
	}

	t.Run("when the grab is refused", func(t *testing.T) {
		g, s, _ := panel("off")
		s.refuse = fmt.Errorf("%w ctrl+alt+f; choose another, or %s", hotkey.ErrHeld, hotkey.Fallback)
		press(t, g, ebiten.KeyEnter)
		press(t, g, ebiten.KeyF, "ctrl", "alt")

		t.Run("it should say so in the panel, with the fallback", func(t *testing.T) {
			assert.Contains(t, g.settings.Note, "botropolis find")
		})
	})

	t.Run("when the desktop refuses the combination pressed", func(t *testing.T) {
		g, s, applied := panel("ctrl+alt+f")
		s.refuse = fmt.Errorf("%w: alt+f needs ctrl or cmd with it on macOS", hotkey.ErrRefused)
		press(t, g, ebiten.KeyEnter)
		press(t, g, ebiten.KeyF, "alt")

		t.Run("it should say why in the panel", func(t *testing.T) {
			assert.Contains(t, g.settings.Note, "needs ctrl or cmd")
		})

		t.Run("it should not save it", func(t *testing.T) {
			assert.Empty(t, saved(applied))
		})
	})

	t.Run("when the city starts with a refused combination set", func(t *testing.T) {
		g, s, _ := panel("alt+f")
		s.refuse = fmt.Errorf("%w: alt+f needs ctrl or cmd with it on macOS", hotkey.ErrRefused)
		g.useShortcut("alt+f")

		t.Run("it should say why in the panel", func(t *testing.T) {
			assert.Contains(t, g.settings.Note, "needs ctrl or cmd")
		})
	})

	t.Run("when the desktop has no way to register one", func(t *testing.T) {
		g, _, _ := panel("off")
		g.shortcut = nil
		press(t, g, ebiten.KeyEnter)

		t.Run("it should say so in the panel, with the fallback", func(t *testing.T) {
			assert.Contains(t, g.settings.Note, "botropolis find")
		})
	})

	t.Run("when the city starts with a combination set", func(t *testing.T) {
		g, s, _ := panel("ctrl+alt+f")
		g.useShortcut("ctrl+alt+f")

		t.Run("it should grab it", func(t *testing.T) {
			require.NotNil(t, s.grabbed)
			assert.Equal(t, "ctrl+alt+f", s.grabbed.String())
		})
	})
}
