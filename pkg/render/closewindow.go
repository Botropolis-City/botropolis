package render

import (
	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/tray"
)

// traySeam is the tray icon while it is up: what its menu was asked.
// Item 293.
type traySeam interface {
	Actions() <-chan tray.Action
}

// noteClose holds the window manager's close request — the X button, or
// i3's kill — until the key path answers it, however many ticks pass
// before it runs.
func (g *Game) noteClose() {
	closing := g.closing
	if closing == nil {
		closing = ebiten.IsWindowBeingClosed
	}
	if closing() {
		g.closeAsked = true
	}
}

// answerClose asks, or hides the city in the tray where on_close says
// so and there is a tray to open it from again.
func (g *Game) answerClose() {
	if !g.closeAsked {
		return
	}
	g.closeAsked = false
	if g.onClose == config.OnCloseTray && g.tray != nil {
		g.ownWindow().Hide()
		return
	}
	g.openQuit()
}

// answerTray takes what the tray's menu was asked: to open the city, or
// to quit it, which shows the window and asks.
func (g *Game) answerTray() {
	if g.tray == nil {
		return
	}
	for {
		select {
		case a := <-g.tray.Actions():
			g.comeForward(0)
			if a == tray.Quit {
				g.openQuit()
			}
		default:
			return
		}
	}
}

// onCloseNote is what the on_close row says under the rows: why tray
// would ask instead, or nothing.
func (g *Game) onCloseNote() string {
	if g.onClose != config.OnCloseTray || g.tray != nil {
		return ""
	}
	why := tray.ErrNoTray.Error()
	if g.trayErr != nil {
		why = g.trayErr.Error()
	}
	return "on close: " + why + "; closing the window asks instead"
}
