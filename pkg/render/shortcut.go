package render

import (
	"errors"
	"fmt"
	"runtime"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/hotkey"
	"botropolis.city/botropolis/pkg/ui"
)

// The global shortcut's row in the settings panel: Enter records the
// next combination pressed, Esc cancels, one that means something in
// other applications is named before it is saved, and Backspace or
// Delete clears it to off. Item 84.

const (
	shortcutOff    = "off"
	shortcutAsking = "press a combination; Esc cancels."
)

// onShortcutRow is whether the settings cursor is on the global
// shortcut's row.
func (g *Game) onShortcutRow() bool {
	items := g.settings.Items
	return g.settings.Cursor < len(items) && items[g.settings.Cursor].Key == config.KeyGlobalShortcut
}

// shortcutRowKeys answers the row's own keys; it reports whether it
// took the key.
func (g *Game) shortcutRowKeys() bool {
	just := g.just
	switch {
	case just(ebiten.KeyBackspace) || just(ebiten.KeyDelete):
		g.clearShortcut()
	case just(ebiten.KeyEnter) || just(ebiten.KeySpace) || just(ebiten.KeyArrowRight) || just(ebiten.KeyL):
		if g.shortcut == nil {
			g.settings.Note = g.unsupported()
			return true
		}
		g.recordingShortcut = true
		g.settings.Note = shortcutAsking
	case just(ebiten.KeyArrowLeft) || just(ebiten.KeyH):
	default:
		return false
	}
	return true
}

// recordShortcutKeys takes every key while the row records or waits to
// be told to save a combination that means something elsewhere; it
// reports whether it took the tick.
func (g *Game) recordShortcutKeys() bool {
	just := g.just
	if g.pendingShortcut != nil {
		switch {
		case just(ebiten.KeyEnter):
			c := *g.pendingShortcut
			g.pendingShortcut = nil
			g.saveShortcut(c)
		case just(ebiten.KeyEscape):
			g.pendingShortcut = nil
			g.settings.Note = ""
		}
		return true
	}
	if !g.recordingShortcut {
		return false
	}
	if just(ebiten.KeyEscape) {
		g.recordingShortcut = false
		g.settings.Note = ""
		return true
	}
	key, ok := g.justPressedKey()
	if !ok {
		return true
	}
	g.recordingShortcut = false
	c := config.Chord{
		Ctrl:  g.scriptedCtrl || ebiten.IsKeyPressed(ebiten.KeyControl),
		Alt:   g.scriptedAlt || ebiten.IsKeyPressed(ebiten.KeyAlt),
		Shift: g.shifted(),
		Cmd:   g.scriptedCmd || ebiten.IsKeyPressed(ebiten.KeyMeta),
		Key:   strings.ToLower(key.String()),
	}
	if meaning := hotkey.CommonMeaning(c); meaning != "" {
		g.pendingShortcut = &c
		g.settings.Note = fmt.Sprintf("%s %s. Enter saves it anyway; Esc cancels.", c, meaning)
		return true
	}
	g.saveShortcut(c)
	return true
}

// justPressedKey is the key that was just pressed, other than a
// modifier, a lock or Esc.
func (g *Game) justPressedKey() (ebiten.Key, bool) {
	for k := ebiten.Key(0); k <= ebiten.KeyMax; k++ {
		if g.just(k) && !modifierKey(k) && k != ebiten.KeyEscape {
			return k, true
		}
	}
	return -1, false
}

func modifierKey(k ebiten.Key) bool {
	switch k {
	case ebiten.KeyAlt, ebiten.KeyAltLeft, ebiten.KeyAltRight,
		ebiten.KeyControl, ebiten.KeyControlLeft, ebiten.KeyControlRight,
		ebiten.KeyShift, ebiten.KeyShiftLeft, ebiten.KeyShiftRight,
		ebiten.KeyMeta, ebiten.KeyMetaLeft, ebiten.KeyMetaRight,
		ebiten.KeyCapsLock, ebiten.KeyNumLock, ebiten.KeyScrollLock:
		return true
	}
	return false
}

// saveShortcut grabs c and saves it; a combination that cannot be
// grabbed is still saved, and the panel says why it does not work,
// except one the desktop refuses outright, which is not saved (item 95).
func (g *Game) saveShortcut(c config.Chord) {
	note, refused := g.grabShortcut(c)
	if !refused {
		g.setShortcutRow(c.String())
	}
	g.settings.Note = note
}

func (g *Game) clearShortcut() {
	if g.shortcut != nil {
		if err := g.shortcut.Release(); err != nil {
			g.SetStatus(err.Error())
		}
	}
	g.setShortcutRow(shortcutOff)
	g.settings.Note = ""
}

func (g *Game) setShortcutRow(value string) {
	for i := range g.settings.Items {
		if g.settings.Items[i].Key == config.KeyGlobalShortcut {
			g.settings.Items[i].Value = value
			g.applySetting(g.settings.Items[i])
			return
		}
	}
	g.applySetting(ui.Setting{Key: config.KeyGlobalShortcut, Label: "global shortcut", Value: value})
}

// grabShortcut grabs c, says in words why it could not, and reports
// whether the desktop refuses c outright.
func (g *Game) grabShortcut(c config.Chord) (string, bool) {
	if g.shortcut == nil {
		return g.unsupported(), false
	}
	if err := g.shortcut.Grab(c); err != nil {
		return err.Error(), errors.Is(err, hotkey.ErrRefused)
	}
	return "", false
}

// useShortcut grabs the combination the config holds as the city
// starts, and says so where it cannot.
func (g *Game) useShortcut(value string) {
	if value == "" {
		return
	}
	c, err := config.ParseChord(config.KeyGlobalShortcut, value, runtime.GOOS, KnownKey)
	note := ""
	if err != nil {
		note = err.Error()
	} else {
		note, _ = g.grabShortcut(c)
	}
	if note != "" {
		g.settings.Note = "global shortcut: " + note
		g.SetStatus(g.settings.Note)
	}
}

func (g *Game) unsupported() string {
	if g.desktopErr != nil {
		return g.desktopErr.Error()
	}
	return fmt.Sprintf("%v; %s", hotkey.ErrUnsupported, hotkey.Fallback)
}
