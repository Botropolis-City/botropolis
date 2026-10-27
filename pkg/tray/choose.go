package tray

import "fmt"

// kind is which tray the desktop has.
type kind int

const (
	kindNone kind = iota
	kindSNI
	kindXEmbed
)

// found is what the probes saw: a StatusNotifierWatcher on the session
// bus and whether a host has registered with it, and a manager owning
// the display's XEmbed system tray selection.
type found struct {
	watcher, host, manager bool
}

// choose prefers StatusNotifierItem, which the panels of KDE, Waybar
// and GNOME's extension host, and falls back to the XEmbed tray of
// i3bar and polybar. A watcher with no host shows nothing, so it counts
// for nothing.
func choose(f found) (kind, error) {
	switch {
	case f.watcher && f.host:
		return kindSNI, nil
	case f.manager:
		return kindXEmbed, nil
	}
	why := "no StatusNotifierItem host is on the session bus"
	if f.watcher {
		why = "a StatusNotifierItem watcher is on the session bus with no host registered"
	}
	return kindNone, fmt.Errorf("%w: %s, and nothing owns the display's XEmbed system tray", ErrNoTray, why)
}
