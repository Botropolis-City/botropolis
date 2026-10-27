// Package tray puts the city in the desktop's tray while it runs: on
// Linux through StatusNotifierItem, falling back to the XEmbed system
// tray, so a window closed to the tray can be opened again (item 293).
package tray

import "errors"

// Action is an entry of the tray's menu, as the city hears it.
type Action int

const (
	// Open shows the city's window: the menu's first entry, and a left
	// click on the icon.
	Open Action = iota
	// Quit shows the window and asks, as q does: always the menu's last
	// entry, so others can go above it.
	Quit
)

// ErrNoTray is a desktop with nothing that hosts a tray.
var ErrNoTray = errors.New("no tray was found")

// Tray is the icon while it is up: what its menu was asked, and taking
// it down.
type Tray interface {
	Actions() <-chan Action
	Close()
}
