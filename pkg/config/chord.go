package config

import (
	"fmt"
	"strings"
)

// The finder's chords, which can be set like any other setting. Item 82.
const (
	KeyFind     = "find_key"
	KeyFindCopy = "find_copy_key"
	KeyFindShow = "find_show_key"
	// KeyNewSession opens New session (item 87).
	KeyNewSession = "new_session_key"

	// DefaultFindShow is Control+M on every system: ⌘M would minimize
	// the window on macOS, where mod is ⌘.
	DefaultFind       = "mod+k"
	DefaultFindCopy   = "mod+y"
	DefaultFindShow   = "ctrl+m"
	DefaultNewSession = "shift+n"
)

// Chord is a key with the modifiers held for it.
type Chord struct {
	Ctrl, Cmd, Alt, Shift bool
	Key                   string
}

// String is the chord the way a setting spells it.
func (c Chord) String() string {
	var parts []string
	for _, m := range []struct {
		held bool
		name string
	}{{c.Ctrl, "ctrl"}, {c.Alt, "alt"}, {c.Shift, "shift"}, {c.Cmd, "cmd"}} {
		if m.held {
			parts = append(parts, m.name)
		}
	}
	return strings.Join(append(parts, c.Key), "+")
}

// Chords are the finder's: the one that opens it, the one that copies
// how to resume a session, and the one that shows it on the map.
type Chords struct{ Open, Copy, Show, New Chord }

// macOSKeeps is what macOS does with the ⌘ chords it takes for itself
// before an application sees them.
var macOSKeeps = map[string]string{
	"m": "minimizes the window",
	"h": "hides the application",
	"q": "quits the application",
	"w": "closes the window",
}

// ParseChord reads a chord such as ctrl+m, alt+shift+s or f3: modifiers
// joined to a key with +, where mod is ⌘ on macOS and Ctrl elsewhere.
// known says whether the key is one the city can read.
func ParseChord(setting, value, goos string, known func(string) bool) (Chord, error) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(value)), "+")
	c := Chord{Key: parts[len(parts)-1]}
	for _, mod := range parts[:len(parts)-1] {
		switch mod {
		case "ctrl", "control":
			c.Ctrl = true
		case "cmd", "command", "super", "meta":
			c.Cmd = true
		case "alt", "option", "opt":
			c.Alt = true
		case "shift":
			c.Shift = true
		case "mod":
			if goos == "darwin" {
				c.Cmd = true
			} else {
				c.Ctrl = true
			}
		default:
			return Chord{}, fmt.Errorf("%s: %q is not a modifier in %q; use ctrl, cmd, alt, shift or mod", setting, mod, value)
		}
	}
	if c.Key == "" || !known(c.Key) {
		return Chord{}, fmt.Errorf("%s: %q is not a key in %q", setting, c.Key, value)
	}
	if does, kept := macOSKeeps[c.Key]; kept && goos == "darwin" && c.Cmd && !c.Ctrl && !c.Alt {
		return Chord{}, fmt.Errorf("%s: %s %s on macOS before the city sees it; choose another chord", setting, value, does)
	}
	return c, nil
}

// FindChords reads the finder's chords from the configuration, with the
// defaults for anything not set.
func FindChords(cfg Config, goos string, known func(string) bool) (Chords, error) {
	var chords Chords
	for _, s := range []struct {
		setting, value, fallback string
		into                     *Chord
	}{
		{KeyFind, cfg.FindKey, DefaultFind, &chords.Open},
		{KeyFindCopy, cfg.FindCopyKey, DefaultFindCopy, &chords.Copy},
		{KeyFindShow, cfg.FindShowKey, DefaultFindShow, &chords.Show},
		{KeyNewSession, cfg.NewSessionKey, DefaultNewSession, &chords.New},
	} {
		value := s.value
		if value == "" {
			value = s.fallback
		}
		c, err := ParseChord(s.setting, value, goos, known)
		if err != nil {
			return Chords{}, err
		}
		*s.into = c
	}
	return chords, nil
}
