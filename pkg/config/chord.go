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
	// KeyExplain is the key held for explain mode (item 88).
	KeyExplain = "explain_key"
	// KeyTriage starts triage (item 88).
	KeyTriage = "triage_key"
	// KeyStream hides and shows the stream (item 88).
	KeyStream = "stream_key"
	// KeyJournal brings the journal back (item 86).
	KeyJournal = "journal_key"
	// KeyCityPlan opens and closes the City Plan (item 106).
	KeyCityPlan = "city_plan_key"
	// KeyLabelsToggle turns every label off and on (item 102).
	KeyLabelsToggle = "labels_key"

	// DefaultFindShow is Control+M on every system: ⌘M would minimize
	// the window on macOS, where mod is ⌘.
	DefaultFind       = "mod+k"
	DefaultFindCopy   = "mod+y"
	DefaultFindShow   = "ctrl+m"
	DefaultNewSession = "shift+n"
	// DefaultExplain is e: Alt, the genre's key for it, is taken by
	// some desktops for moving windows.
	DefaultExplain = "e"
	// DefaultTriage is w, for walk: tab and shift+tab already mean
	// the next and the previous session needing you.
	DefaultTriage = "w"
	// DefaultStream is l, for log.
	DefaultStream = "l"
	// DefaultJournal is j: a place takes a bare letter, and j was free
	// at the map (item 350). It was shift+w, for where was I.
	DefaultJournal = "j"
	// DefaultCityPlan is m, for map.
	DefaultCityPlan = "m"
	// DefaultLabelsToggle is z, the one letter left free.
	DefaultLabelsToggle = "z"
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
// Bound is every binding in Bindings by name, these among them
// (item 350).
type Chords struct {
	Open, Copy, Show, New, Explain, Triage, Stream, Journal, Plan, Labels Chord
	Bound                                                                 map[string]Chord
}

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
	bound, err := LoadBindings(cfg, goos, known)
	if err != nil {
		return Chords{}, err
	}
	return chordsOf(bound), nil
}

// chordsOf is the chords read from the one map of bindings, so the
// named ones cannot drift from it (item 350).
func chordsOf(bound map[string]Chord) Chords {
	return Chords{
		Open: bound["find"], Copy: bound["find_copy"], Show: bound["find_show"], New: bound["new_session"],
		Explain: bound["explain"], Triage: bound["triage"], Stream: bound["stream"], Journal: bound["journal"],
		Plan: bound["city_plan"], Labels: bound["labels"], Bound: bound,
	}
}

// With is the chords with one binding rebound, the rest as they were;
// the chords it is made from are left alone.
func (c Chords) With(name string, chord Chord) Chords {
	bound := make(map[string]Chord, len(c.Bound)+1)
	for k, v := range c.Bound {
		bound[k] = v
	}
	bound[name] = chord
	return chordsOf(bound)
}

// Rebind checks a chord for a binding as a config file's would be: a
// chord macOS keeps, a key that stays fixed where the binding answers,
// and a chord another binding answers together with it are refused,
// each saying why.
func Rebind(c Chords, name string, chord Chord, goos string) error {
	known := false
	for _, b := range Bindings {
		known = known || b.Name == name
	}
	if !known {
		return fmt.Errorf("no binding is called %s", name)
	}
	if does, kept := macOSKeeps[chord.Key]; kept && goos == "darwin" && chord.Cmd && !chord.Ctrl && !chord.Alt {
		return fmt.Errorf("%s %s on macOS before the city sees it", chord.Label(), does)
	}
	return clashes(c.With(name, chord).Bound)
}
