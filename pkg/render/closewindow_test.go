package render

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/tray"
	"botropolis.city/botropolis/pkg/ui"
)

// fakeTray is a tray whose menu a test clicks by sending it an action.
type fakeTray struct{ actions chan tray.Action }

func (f *fakeTray) Actions() <-chan tray.Action { return f.actions }

// Item 293. The window manager's close request — the X button, or i3's
// kill — closed the window and the city with it, without asking.
func TestCloseWindow(t *testing.T) {
	noTray := errors.New("no tray: nothing on the session bus or the display hosts one")

	type rig struct {
		g   *Game
		win *fakeWindow
		err error
	}
	newRig := func(onClose string, withTray bool) *rig {
		r := &rig{win: &fakeWindow{}}
		r.g = &Game{scene: city.NewScene(city.NewLayout()), scripted: -1, clicked: -1, win: r.win, onClose: onClose}
		if withTray {
			r.g.tray = &fakeTray{actions: make(chan tray.Action, 1)}
		} else {
			r.g.trayErr = noTray
		}
		return r
	}
	// closed is a city that hears one close request, on its next tick.
	closed := func(t *testing.T, onClose string, withTray bool) *rig {
		t.Helper()
		r := newRig(onClose, withTray)
		asked := true
		r.g.closing = func() bool { defer func() { asked = false }(); return asked }
		r.g.noteClose()
		r.err = r.g.handleKeys()
		return r
	}
	quitOpen := func(g *Game) bool {
		_, ok := g.overlays.top().(*quitPanel)
		return ok
	}

	t.Run("when the window manager asks to close the window", func(t *testing.T) {
		t.Run("and on_close is ask", func(t *testing.T) {
			r := closed(t, config.OnCloseAsk, true)

			t.Run("it should open the quit panel", func(t *testing.T) {
				assert.True(t, quitOpen(r.g))
			})

			t.Run("it should not leave", func(t *testing.T) {
				assert.NoError(t, r.err)
			})
		})

		t.Run("and on_close is ask, and another panel is open", func(t *testing.T) {
			r := newRig(config.OnCloseAsk, true)
			r.g.openReference()
			r.g.closing = func() bool { return true }
			r.g.noteClose()
			require.NoError(t, r.g.handleKeys())

			t.Run("it should put the quit panel on top", func(t *testing.T) {
				assert.True(t, quitOpen(r.g))
			})
		})

		t.Run("and on_close is tray, and a tray is present", func(t *testing.T) {
			r := closed(t, config.OnCloseTray, true)

			t.Run("it should hide the window", func(t *testing.T) {
				assert.True(t, r.win.hidden)
			})

			t.Run("it should not leave", func(t *testing.T) {
				assert.NoError(t, r.err)
			})

			t.Run("it should not open the quit panel", func(t *testing.T) {
				assert.False(t, quitOpen(r.g))
			})
		})

		t.Run("and on_close is tray, and no tray was found", func(t *testing.T) {
			r := closed(t, config.OnCloseTray, false)

			t.Run("it should open the quit panel", func(t *testing.T) {
				assert.True(t, quitOpen(r.g))
			})
		})

		t.Run("and it is answered", func(t *testing.T) {
			r := closed(t, config.OnCloseAsk, true)
			r.g.standQuitDown()
			require.NoError(t, r.g.handleKeys())

			t.Run("it should not ask again", func(t *testing.T) {
				assert.False(t, quitOpen(r.g))
			})
		})
	})

	// clicked is a hidden city whose tray menu was clicked once.
	clicked := func(t *testing.T, a tray.Action) *rig {
		t.Helper()
		r := newRig(config.OnCloseTray, true)
		r.win.hidden = true
		r.g.tray.(*fakeTray).actions <- a
		r.err = r.g.handleKeys()
		return r
	}

	t.Run("when the tray asks to open the city", func(t *testing.T) {
		t.Run("and the window is hidden", func(t *testing.T) {
			r := clicked(t, tray.Open)

			t.Run("it should show the window", func(t *testing.T) {
				assert.False(t, r.win.hidden)
			})

			t.Run("it should not open the quit panel", func(t *testing.T) {
				assert.False(t, quitOpen(r.g))
			})
		})
	})

	t.Run("when the tray asks to quit", func(t *testing.T) {
		t.Run("and the window is hidden", func(t *testing.T) {
			r := clicked(t, tray.Quit)

			t.Run("it should show the window", func(t *testing.T) {
				assert.False(t, r.win.hidden)
			})

			t.Run("it should open the quit panel", func(t *testing.T) {
				assert.True(t, quitOpen(r.g))
			})

			t.Run("it should not leave", func(t *testing.T) {
				assert.NoError(t, r.err)
			})
		})
	})

	t.Run("when the on_close row is set to tray", func(t *testing.T) {
		row := ui.Setting{Key: config.KeyOnClose, Label: "on close", Options: []string{config.OnCloseAsk, config.OnCloseTray}, Value: config.OnCloseTray}

		t.Run("and no tray was found", func(t *testing.T) {
			r := newRig(config.OnCloseAsk, false)
			r.g.applySetting(row)

			t.Run("it should say why under the rows", func(t *testing.T) {
				assert.Contains(t, r.g.settings.Note, noTray.Error())
			})

			t.Run("it should still take the setting", func(t *testing.T) {
				assert.Equal(t, config.OnCloseTray, r.g.onClose)
			})
		})

		t.Run("and a tray is present", func(t *testing.T) {
			r := newRig(config.OnCloseAsk, true)
			r.g.applySetting(row)

			t.Run("it should say nothing under the rows", func(t *testing.T) {
				assert.Empty(t, r.g.settings.Note)
			})
		})
	})

}
