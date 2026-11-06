package render

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 350, slice 5b: the settings' keys… row opens the Keys panel,
// which records a key for a binding, refuses one another binding or the
// city's grammar holds, puts one back with Backspace, and saves it.
func TestKeysPanel(t *testing.T) {
	type rig struct {
		*buttonPanRig
		saved []ui.Setting
	}
	opened := func(t *testing.T) *rig {
		t.Helper()
		r := &rig{buttonPanRig: newButtonPanRig(t)}
		r.g.apply = func(s ui.Setting) error {
			r.saved = append(r.saved, s)
			return nil
		}
		r.g.settings = ui.Settings{Items: []ui.Setting{{Key: config.KeyDetail, Label: "detail", Options: []string{"plain", "rich"}, Value: "plain"}, KeysRow}, Cursor: 1}
		r.g.settingsOpen = true
		keyPress(t, r.g, ebiten.KeyEnter, false)
		return r
	}
	panelOf := func(g *Game) *keysPanel {
		p, _ := g.overlays.top().(*keysPanel)
		return p
	}
	to := func(t *testing.T, g *Game, name string) {
		t.Helper()
		p := panelOf(g)
		require.NotNil(t, p)
		for i, row := range p.rows {
			if row.name == name {
				p.focus = i
				return
			}
		}
		require.Failf(t, "no row", "%s", name)
	}
	saved := func(r *rig, key string) (string, bool) {
		for i := len(r.saved) - 1; i >= 0; i-- {
			if r.saved[i].Key == key {
				return r.saved[i].Value, true
			}
		}
		return "", false
	}

	t.Run("when the settings' keys row is chosen", func(t *testing.T) {
		r := opened(t)

		t.Run("it should open the Keys panel on its first row", func(t *testing.T) {
			p := panelOf(r.g)
			require.NotNil(t, p)
			assert.Equal(t, 0, p.focus)
		})

		t.Run("it should list every binding once", func(t *testing.T) {
			assert.Len(t, panelOf(r.g).rows, len(config.Bindings))
		})

		for name, key := range map[string]ebiten.Key{"down": ebiten.KeyArrowDown, "j": ebiten.KeyJ} {
			t.Run("and "+name+" is pressed", func(t *testing.T) {
				r := opened(t)
				keyPress(t, r.g, key, false)

				t.Run("it should focus the next row", func(t *testing.T) {
					assert.Equal(t, 1, panelOf(r.g).focus)
				})
			})
		}

		t.Run("and esc is pressed", func(t *testing.T) {
			r := opened(t)
			keyPress(t, r.g, ebiten.KeyEscape, false)

			t.Run("it should return to the settings", func(t *testing.T) {
				assert.True(t, panelOf(r.g) == nil && r.g.settingsOpen)
			})
		})
	})

	t.Run("when the timeline's row records y", func(t *testing.T) {
		r := opened(t)
		to(t, r.g, "timeline")
		keyPress(t, r.g, ebiten.KeyEnter, false)
		keyPress(t, r.g, ebiten.KeyY, false)

		t.Run("it should save keys.timeline as y", func(t *testing.T) {
			v, ok := saved(r, config.KeySettings+".timeline")
			assert.True(t, ok && v == "y", "%v", r.saved)
		})

		t.Run("it should bind the timeline to y at once", func(t *testing.T) {
			assert.Equal(t, config.Chord{Key: "y"}, r.g.findChords().Bound["timeline"])
		})

		t.Run("it should show y on the row", func(t *testing.T) {
			p := panelOf(r.g)
			assert.Equal(t, "y", p.keyRows(r.g)[p.focus].Key)
		})
	})

	t.Run("when the timeline's row records x, the breakdown's", func(t *testing.T) {
		r := opened(t)
		to(t, r.g, "timeline")
		keyPress(t, r.g, ebiten.KeyEnter, false)
		keyPress(t, r.g, ebiten.KeyX, false)

		t.Run("it should save nothing", func(t *testing.T) {
			assert.Empty(t, r.saved)
		})

		t.Run("it should say on the row that the breakdown holds it", func(t *testing.T) {
			p := panelOf(r.g)
			assert.Contains(t, p.keyRows(r.g)[p.focus].Refused, "breakdown")
		})

		t.Run("it should keep t", func(t *testing.T) {
			assert.Equal(t, config.Chord{Key: "t"}, r.g.findChords().Bound["timeline"])
		})
	})

	t.Run("when a row is recording and esc is pressed", func(t *testing.T) {
		r := opened(t)
		to(t, r.g, "timeline")
		keyPress(t, r.g, ebiten.KeyEnter, false)
		keyPress(t, r.g, ebiten.KeyEscape, false)

		t.Run("it should record nothing, and stay open", func(t *testing.T) {
			assert.True(t, len(r.saved) == 0 && panelOf(r.g) != nil && !panelOf(r.g).recording)
		})
	})

	t.Run("when a rebound row is put back with backspace", func(t *testing.T) {
		r := opened(t)
		to(t, r.g, "timeline")
		keyPress(t, r.g, ebiten.KeyEnter, false)
		keyPress(t, r.g, ebiten.KeyY, false)
		keyPress(t, r.g, ebiten.KeyBackspace, false)

		t.Run("it should save it as its default, which is no line", func(t *testing.T) {
			v, ok := saved(r, config.KeySettings+".timeline")
			assert.True(t, ok && v == "", "%v", r.saved)
		})

		t.Run("it should bind it to its default at once", func(t *testing.T) {
			assert.Equal(t, config.Chord{Key: "t"}, r.g.findChords().Bound["timeline"])
		})
	})

	t.Run("when a row the shift key starts records shift+y", func(t *testing.T) {
		r := opened(t)
		to(t, r.g, "close_up")
		keyPress(t, r.g, ebiten.KeyEnter, false)
		keyPress(t, r.g, ebiten.KeyY, true)

		t.Run("it should record the shift with it", func(t *testing.T) {
			assert.Equal(t, config.Chord{Shift: true, Key: "y"}, r.g.findChords().Bound["close_up"])
		})
	})
}

// keyPress is one frame of the keys with key pressed, and shift held
// with it when asked.
func keyPress(t *testing.T, g *Game, key ebiten.Key, shift bool) {
	t.Helper()
	g.scripted, g.scriptedShift = key, shift
	require.NoError(t, g.handleKeys())
	g.scripted, g.scriptedShift = -1, false
}
