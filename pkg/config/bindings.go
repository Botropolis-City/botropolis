package config

import (
	"fmt"
	"slices"
	"strings"
)

// Every key the city answers that a person can rebind, in one table
// (item 350): the map's places, the action menu's column and row
// letters, and the finder's chords. The pointer, the wheel, the camera's
// arrows and +/-, esc, enter and tab, F1, d d and delete delete, and the
// keys that move inside a list are the city's grammar and stay fixed.

// Scope is where a binding answers. Two bindings clash when they share
// a chord in one scope, or when one of them answers everywhere.
type Scope string

const (
	ScopeEverywhere  Scope = "everywhere"
	ScopeMap         Scope = "map"
	ScopeFinder      Scope = "finder"
	ScopeMenu        Scope = "menu"
	ScopeMenuSession Scope = "menu:session"
	ScopeMenuProject Scope = "menu:project"
	ScopeMenuGroup   Scope = "menu:group"
)

// Binding is one rebindable key: its name under keys in the config
// file, where it answers, its default, and the older setting that set it
// before the table, if one did.
type Binding struct {
	Name    string
	Scope   Scope
	Default string
	Legacy  string
}

// KeySettings is where a config file sets bindings: keys: {name: chord}.
const KeySettings = "keys"

// Bindings is the table. Its order is the reference's.
var Bindings = []Binding{
	{"find", ScopeEverywhere, DefaultFind, KeyFind},
	{"find_copy", ScopeFinder, DefaultFindCopy, KeyFindCopy},
	{"find_show", ScopeFinder, DefaultFindShow, KeyFindShow},

	{"details", ScopeMap, "i", ""},
	{"read", ScopeMap, "o", ""},
	{"triage", ScopeMap, DefaultTriage, KeyTriage},
	{"clinic", ScopeMap, "a", ""},
	{"journal", ScopeMap, DefaultJournal, KeyJournal},
	{"stream", ScopeMap, DefaultStream, KeyStream},
	{"lenses", ScopeMap, "v", ""},
	{"lens_1", ScopeMap, "digit1", ""},
	{"lens_2", ScopeMap, "digit2", ""},
	{"lens_3", ScopeMap, "digit3", ""},
	{"lens_4", ScopeMap, "digit4", ""},
	{"lens_5", ScopeMap, "digit5", ""},
	{"lens_6", ScopeMap, "digit6", ""},
	{"new_here", ScopeMap, "c", ""},
	{"fit", ScopeMap, "f", ""},
	{"fit_zero", ScopeMap, "digit0", ""},
	{"city_plan", ScopeMap, DefaultCityPlan, KeyCityPlan},
	{"labels", ScopeMap, DefaultLabelsToggle, KeyLabelsToggle},
	{"turn", ScopeMap, "r", ""},
	{"light", ScopeMap, "n", ""},
	{"clock_back", ScopeMap, "bracketleft", ""},
	{"clock_on", ScopeMap, "bracketright", ""},
	{"settings", ScopeMap, "s", ""},
	{"breakdown", ScopeMap, "x", ""},
	{"usage", ScopeMap, "u", ""},
	{"board", ScopeMap, "g", ""},
	{"new_session", ScopeMap, DefaultNewSession, KeyNewSession},
	{"timeline", ScopeMap, "t", ""},
	{"sidebar", ScopeMap, "b", ""},
	{"hide", ScopeMap, "h", ""},
	{"screenshot", ScopeMap, "p", ""},
	{"share", ScopeMap, "shift+s", ""},
	{"close_up", ScopeMap, "shift+q", ""},
	{"finder_slash", ScopeMap, "slash", ""},
	{"reference", ScopeMap, "shift+slash", ""},
	{"explain", ScopeMap, DefaultExplain, KeyExplain},
	{"quit", ScopeMap, "q", ""},
	{"action_menu", ScopeMap, "space", ""},

	{"menu_session", ScopeMenu, "s", ""},
	{"menu_project", ScopeMenu, "p", ""},
	{"menu_group", ScopeMenu, "g", ""},

	{"menu_attach", ScopeMenuSession, "a", ""},
	{"menu_stop", ScopeMenuSession, "s", ""},
	{"menu_read", ScopeMenuSession, "r", ""},
	{"menu_generate", ScopeMenuSession, "g", ""},
	{"menu_fork", ScopeMenuSession, "f", ""},
	{"menu_pack", ScopeMenuSession, "p", ""},
	{"menu_breakdown", ScopeMenuSession, "b", ""},
	{"menu_open_pr", ScopeMenuSession, "o", ""},
	{"menu_demolish", ScopeMenuSession, "d", ""},

	{"menu_new_here", ScopeMenuProject, "n", ""},
	{"menu_reveal", ScopeMenuProject, "f", ""},
	{"menu_copy_path", ScopeMenuProject, "c", ""},
	{"menu_star", ScopeMenuProject, "s", ""},
	{"menu_hide", ScopeMenuProject, "h", ""},
	{"menu_district_type", ScopeMenuProject, "d", ""},
	{"menu_show_road", ScopeMenuProject, "r", ""},

	{"menu_move", ScopeMenuGroup, "m", ""},
	{"menu_join", ScopeMenuGroup, "a", ""},
	{"menu_take_out", ScopeMenuGroup, "t", ""},
	{"menu_rename_group", ScopeMenuGroup, "n", ""},
	{"menu_close_group", ScopeMenuGroup, "c", ""},
	{"menu_standup", ScopeMenuGroup, "s", ""},
	{"menu_mute", ScopeMenuGroup, "w", ""},
	{"menu_focus", ScopeMenuGroup, "f", ""},
}

// legacyValue is what an older *_key setting holds, and its default,
// for the bindings that had one.
func legacyValue(cfg Config, setting string) (value, fallback string) {
	switch setting {
	case KeyFind:
		return cfg.FindKey, DefaultFind
	case KeyFindCopy:
		return cfg.FindCopyKey, DefaultFindCopy
	case KeyFindShow:
		return cfg.FindShowKey, DefaultFindShow
	case KeyNewSession:
		return cfg.NewSessionKey, DefaultNewSession
	case KeyExplain:
		return cfg.ExplainKey, DefaultExplain
	case KeyTriage:
		return cfg.TriageKey, DefaultTriage
	case KeyStream:
		return cfg.StreamKey, DefaultStream
	case KeyJournal:
		return cfg.JournalKey, DefaultJournal
	case KeyCityPlan:
		return cfg.CityPlanKey, DefaultCityPlan
	case KeyLabelsToggle:
		return cfg.LabelsKey, DefaultLabelsToggle
	}
	return "", ""
}

// LoadBindings reads every binding: from keys in the config file, else from its
// older *_key setting, else its default. It refuses a name keys does not
// know, an older setting and keys that disagree, a chord macOS keeps,
// and two bindings that would answer one chord together.
func LoadBindings(cfg Config, goos string, known func(string) bool) (map[string]Chord, error) {
	byName := map[string]Binding{}
	for _, b := range Bindings {
		byName[b.Name] = b
	}
	var unknown []string
	for name := range cfg.Keys {
		if _, ok := byName[name]; !ok {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		slices.Sort(unknown)
		return nil, fmt.Errorf("%s: no binding is called %s", KeySettings, strings.Join(unknown, ", "))
	}
	chords := map[string]Chord{}
	for _, b := range Bindings {
		setting, value := KeySettings+"."+b.Name, b.Default
		// A keys entry at its default is no setting: the config's
		// defaults hold every binding's, so an older *_key still counts.
		set, inKeys := cfg.Keys[b.Name]
		inKeys = inKeys && set != b.Default
		old, oldDefault := legacyValue(cfg, b.Legacy)
		oldSet := b.Legacy != "" && old != "" && old != oldDefault
		switch {
		case inKeys && oldSet:
			mine, err := ParseChord(setting, set, goos, known)
			if err != nil {
				return nil, err
			}
			theirs, err := ParseChord(b.Legacy, old, goos, known)
			if err != nil {
				return nil, err
			}
			if mine != theirs {
				return nil, fmt.Errorf("%s is %q and %s is %q: they set one key, so keep one of them", b.Legacy, old, setting, set)
			}
			value = set
		case inKeys:
			value = set
		case oldSet:
			setting, value = b.Legacy, old
		}
		c, err := ParseChord(setting, value, goos, known)
		if err != nil {
			return nil, err
		}
		chords[b.Name] = c
	}
	return chords, clashes(chords)
}

// fixedAtMap and fixedInMenu are the keys that stay what they are, by
// what they do: a binding cannot take one where it answers.
var (
	fixedAtMap = map[string]string{
		"d": "demolish (d d)", "delete": "delete delete", "tab": "the next needs-you", "enter": "the card's first button",
		"escape": "back", "arrowup": "pan", "arrowdown": "pan", "arrowleft": "pan", "arrowright": "pan",
		"equal": "zoom", "minus": "zoom", "f1": "the reference",
	}
	fixedInMenu = map[string]string{
		"j": "down", "k": "up", "arrowup": "up", "arrowdown": "down", "arrowleft": "the column before",
		"arrowright": "the next column", "tab": "the next column", "enter": "choose", "escape": "back",
	}
)

// fixed is what a bare key already does where a binding answers, if it
// is one that stays fixed there.
func fixed(scope Scope, c Chord) (string, bool) {
	if c.Ctrl || c.Alt || c.Shift || c.Cmd {
		return "", false
	}
	var sets []map[string]string
	switch scope {
	case ScopeMap:
		sets = []map[string]string{fixedAtMap}
	case ScopeMenu, ScopeMenuSession, ScopeMenuProject, ScopeMenuGroup:
		sets = []map[string]string{fixedInMenu}
	case ScopeEverywhere:
		sets = []map[string]string{fixedAtMap, fixedInMenu}
	}
	for _, set := range sets {
		if does, ok := set[c.Key]; ok {
			return does, true
		}
	}
	return "", false
}

// clashes refuses a binding on a key that stays fixed where it answers,
// and two bindings that answer one chord together: in one scope, or with
// one that answers everywhere.
func clashes(chords map[string]Chord) error {
	for _, b := range Bindings {
		if does, ok := fixed(b.Scope, chords[b.Name]); ok {
			return fmt.Errorf("%s is %s, which is fixed as %s there: give it another key", b.Name, chords[b.Name].Label(), does)
		}
	}
	for i, a := range Bindings {
		for _, b := range Bindings[i+1:] {
			together := a.Scope == b.Scope || a.Scope == ScopeEverywhere || b.Scope == ScopeEverywhere
			if together && chords[a.Name] == chords[b.Name] {
				return fmt.Errorf("%s and %s are both %s: give one of them another key", a.Name, b.Name, chords[a.Name].Label())
			}
		}
	}
	return nil
}

// keyLabels is how a key the city names one way reads on screen.
var keyLabels = map[string]string{
	"slash": "/", "bracketleft": "[", "bracketright": "]",
	"digit0": "0", "digit1": "1", "digit2": "2", "digit3": "3", "digit4": "4",
	"digit5": "5", "digit6": "6", "digit7": "7", "digit8": "8", "digit9": "9",
}

// Label is the chord as the reference and a cap show it: "/" for slash,
// "3" for digit3, and "?" for shift and slash.
func (c Chord) Label() string {
	if c.Shift && !c.Ctrl && !c.Alt && !c.Cmd && c.Key == "slash" {
		return "?"
	}
	shown := c
	if label, ok := keyLabels[c.Key]; ok {
		shown.Key = label
	}
	return shown.String()
}

// bindingHelp is a binding's line in botropolis-config(5): where it
// answers and what it is called, with ? in the city naming what each
// does.
func bindingHelp(name string) string {
	for _, b := range Bindings {
		if b.Name != name {
			continue
		}
		what := strings.ReplaceAll(strings.TrimPrefix(b.Name, "menu_"), "_", " ")
		switch b.Scope {
		case ScopeEverywhere:
			return "the key that opens the finder from anywhere in the city"
		case ScopeFinder:
			return "the finder's key for " + what
		case ScopeMenu:
			return "the action menu's key for its " + what + " column"
		case ScopeMenuSession, ScopeMenuProject, ScopeMenuGroup:
			return "the action menu's key for " + what
		}
		return "the key at the map for " + what + "; ? in the city says what it does"
	}
	return ""
}

// SaveBinding writes one binding into the config file at path: under
// keys, dropping the older *_key it replaces, or with chord empty, back
// to its default, which is no line at all (item 350).
func SaveBinding(path, name, chord string) error {
	for _, b := range Bindings {
		if b.Name != name {
			continue
		}
		if b.Legacy != "" {
			if err := Delete(path, b.Legacy); err != nil {
				return err
			}
		}
		if chord == "" {
			return Delete(path, KeySettings+"."+name)
		}
		return Save(path, KeySettings+"."+name, chord)
	}
	return fmt.Errorf("no binding is called %s", name)
}
