// Package hotkey registers the global shortcut that opens the finder
// from any application and brings the city's window forward, on the
// desktops that have a way to: X11 today. It grabs nothing until a
// combination is set. Item 84.
package hotkey

import (
	"errors"
	"fmt"

	"botropolis.city/botropolis/pkg/config"
)

// Fallback is what to do where the shortcut cannot be registered.
const Fallback = "bind `botropolis find` in the desktop's own keyboard settings"

var (
	// ErrUnsupported is a desktop with no way to register a shortcut.
	ErrUnsupported = errors.New("this desktop has no way for botropolis to register a global shortcut")
	// ErrHeld is a combination another application already holds.
	ErrHeld = errors.New("another application holds")
)

// Desktop is the shortcut's seam: one combination grabbed at a time,
// the times it was pressed at, and the window brought forward.
type Desktop interface {
	// Grab replaces whatever was grabbed with c.
	Grab(c config.Chord) error
	// Release lets go of the grab, at once.
	Release() error
	// Fired delivers the server time of each press.
	Fired() <-chan uint32
	// Raise brings this process's window forward, with the time of the
	// press that asked for it, or 0 for none; it reports whether it could.
	Raise(time uint32) bool
	Close()
}

// Open finds the desktop's way to register a shortcut, or says there
// is none, with the fallback.
func Open(goos string, getenv func(string) string) (Desktop, error) {
	switch goos {
	case "linux", "freebsd", "openbsd", "netbsd", "dragonfly":
		if display := getenv("DISPLAY"); display != "" {
			return openX11(display)
		}
	}
	return nil, fmt.Errorf("%w; %s", ErrUnsupported, Fallback)
}

// common is what a chord already means in most applications, for the
// settings panel to name before it is saved.
var common = map[string]string{
	"ctrl+k":       "inserts a link or opens search in many applications",
	"ctrl+p":       "prints, or opens a file, in many applications",
	"ctrl+shift+p": "opens the command palette in editors, and a private window in browsers",
	"ctrl+a":       "selects everything",
	"ctrl+c":       "copies",
	"ctrl+v":       "pastes",
	"ctrl+x":       "cuts",
	"ctrl+z":       "undoes",
	"ctrl+s":       "saves",
	"ctrl+f":       "finds in the page",
	"ctrl+n":       "opens a new window",
	"ctrl+o":       "opens a file",
	"ctrl+q":       "quits",
	"ctrl+w":       "closes the tab",
	"ctrl+t":       "opens a new tab",
	"ctrl+r":       "reloads",
	"ctrl+l":       "goes to the address bar, or clears the terminal",
	"alt+tab":      "switches windows",
	"alt+f4":       "closes the window",
}

// CommonMeaning names what c already means in other applications, or
// nothing.
func CommonMeaning(c config.Chord) string {
	if !c.Ctrl && !c.Alt && !c.Cmd {
		return "is typed in every application without a modifier"
	}
	return common[c.String()]
}

func grabError(c config.Chord, err error) error {
	var access xprotoAccess
	if errors.As(err, &access) {
		return fmt.Errorf("%w %s; choose another, or %s", ErrHeld, c, Fallback)
	}
	return fmt.Errorf("%s: %w", c, err)
}
