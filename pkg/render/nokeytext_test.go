package render

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/stats"
	"botropolis.city/botropolis/pkg/ui"
)

// fixedKeyNames is the keys the list vocabulary and the camera use,
// which are not in config.Bindings, as text would spell them.
var fixedKeyNames = []string{"esc", "escape", "F1", "F2", "PgUp", "PgDn", "↑", "↓", "←", "→", "↑↓", "←→", "j/k", "1-9", "1–9", "+", "-"}

// wordKeys are keys whose names are also words — "delete it" is a
// label and "enter focuses it" a key — so they count only in a key's
// grammar: asked to be pressed, followed by two spaces, or followed by
// what it does.
var wordKeys = map[string]bool{"enter": true, "return": true, "tab": true, "space": true, "delete": true, "backspace": true}

// keyNames is every key a label could spell outside a cap: each
// binding's default as a cap shows it, and the fixed vocabulary. A lone
// letter or digit is left out, since "a" is a word and "3 groups" a
// count; a letter spelled as a key is caught by the key patterns
// instead, and the digits by "1-9".
func keyNames() []string {
	names := append([]string(nil), fixedKeyNames...)
	for _, b := range config.Bindings {
		label := defaultChords().Bound[b.Name].Label()
		if wordKeys[label] {
			continue
		}
		if len([]rune(label)) > 1 || !isLetter(label) && !isDigit(label) {
			names = append(names, label)
		}
	}
	return names
}

func isDigit(s string) bool {
	r := []rune(s)
	return len(r) == 1 && r[0] >= '0' && r[0] <= '9'
}

// keyGrammar is whether the word at i is used as a key: asked to be
// pressed, set before two spaces, in parentheses, or followed by what it does — a verb
// in the third person, "to", "for" or "or".
func keyGrammar(text string, words []string, i int, word string) bool {
	if i > 0 && strings.EqualFold(words[i-1], "press") {
		return true
	}
	if strings.HasPrefix(words[i], "(") && strings.HasSuffix(words[i], ")") {
		return true
	}
	if strings.HasPrefix(text, words[i]+"  ") || strings.Contains(text, " "+words[i]+"  ") {
		return true
	}
	if i+1 < len(words) {
		next := strings.ToLower(strings.Trim(words[i+1], ",.;:"))
		if next == "to" || next == "for" || next == "or" || next == "again" || len(next) > 2 && strings.HasSuffix(next, "s") && !strings.HasSuffix(next, "ss") {
			return true
		}
	}
	return false
}

func isLetter(s string) bool {
	r := []rune(s)
	return len(r) == 1 && (r[0] >= 'a' && r[0] <= 'z' || r[0] >= 'A' && r[0] <= 'Z')
}

// spelledKeys is the key names a text spells as words, and any letter
// it asks to be pressed.
func spelledKeys(text string, names []string) []string {
	known := map[string]bool{}
	for _, n := range names {
		known[n] = true
	}
	var found []string
	words := strings.Fields(text)
	for i, w := range words {
		word := strings.Trim(w, ",.;:()\"'·")
		if known[word] || strings.HasPrefix(word, "shift+") || strings.HasPrefix(word, "ctrl+") || strings.HasPrefix(word, "alt+") || strings.HasPrefix(word, "cmd+") {
			found = append(found, word)
			continue
		}
		if (isLetter(word) || wordKeys[strings.ToLower(word)]) && keyGrammar(text, words, i, word) {
			found = append(found, word)
		}
	}
	return found
}

// keyedTexts is every label and line drawn beside a cap or a button:
// the card's face, the menu's titles, rows and descriptions, and every
// dialog's title, lines and buttons. Item 351's slice 3 adds every
// panel's header.
func keyedTexts() map[string]string {
	texts := map[string]string{}
	for _, col := range city.VerbColumns {
		texts["the "+col.Title+" button"] = col.Title + " " + city.MenuMark
		texts["the "+col.Title+" column's title"] = col.Title
		for _, k := range col.Kinds {
			texts["the "+string(k)+" row"] = string(k)
			texts["what "+string(k)+" does"] = k.Describe()
		}
	}
	texts["what the details fold does"] = city.ActionDetails.Describe()
	texts["what hiding the details does"] = city.ActionDetailsHide.Describe()
	for _, d := range []*dialog{quitDialog(), demolishDialog("Fix the queue", "a"), closeGroupDialog("PROJ-9", "g1")} {
		texts["the dialog "+d.title+"'s title"] = d.title
		for i, line := range d.lines {
			texts["the dialog "+d.title+"'s line "+string(rune('1'+i))] = line
		}
		for _, c := range d.choices {
			texts["the dialog "+d.title+"'s button "+c.label] = c.label
		}
	}
	for what, text := range headerTexts() {
		texts[what] = text
	}
	return texts
}

// headerTexts is every panel's header and line, and every label a
// prompt or a walk card puts beside a key (item 351, slice 4).
func headerTexts() map[string]string {
	texts := map[string]string{
		"the board's header with one group":    boardStatus(make([]boardRow, 1)),
		"the board's header with three groups": boardStatus(make([]boardRow, 3)),
		"the clinic's line":                    clinicStatus(2),
		"the group picker's title":             (&picker{}).title(),
		"the group picker's title, naming":     (&picker{naming: true}).title(),
		"the group picker's title, renaming":   (&picker{renaming: "g1"}).title(),
		"the mute picker's title":              muteTitle,
		"the finder's count":                   finderCount(1, 2, 3),
		"the Keys row in settings":             KeysRow.Value,
		"a packed result in the finder":        ui.PackedWhere,
		"the journal's foot":                   journalFoot("restored 2 sessions"),
		"City Hall's tidy-up button":           boardTidyLabel,
		"City Hall's new-session button":       boardNewLabel,
		"City Hall's pack-up button, at none":  quietLabel(0),
		"City Hall's pack-up button, at three": quietLabel(3),
	}
	for panel, about := range ui.PanelAbout {
		texts["the "+panel+" panel's line"] = about
	}
	for i, set := range walkButtonSets {
		for _, b := range set {
			texts["walk card "+string(rune('1'+i))+"'s button "+b.label] = b.label
		}
	}
	prompts := map[string]statsWording{
		"the statistics-off prompt":         (&statsPrompt{kind: stats.OffPrompt}).wording(),
		"the statistics-on prompt":          (&statsPrompt{kind: stats.OnPrompt}).wording(),
		"the statistics-off prompt at open": (&statsPrompt{kind: stats.OffPrompt, atOpen: true}).wording(),
		"the store notice":                  (&storeNotice{}).wording(),
	}
	for what, w := range prompts {
		texts[what+"'s title"] = w.Title
		for i, line := range w.Lines {
			texts[what+"'s line "+string(rune('1'+i))] = line
		}
		for _, k := range w.Keys {
			texts[what+"'s choice "+k.Action] = k.Action
		}
	}
	for _, k := range (&shareMode{}).keys() {
		texts["the share bar's "+k.Action] = k.Action
	}
	texts["the share bar's title"] = shareTitle
	for _, k := range newSessionKeys {
		texts["new session's "+k.Action] = k.Action
	}
	return texts
}

// Item 351: a key is shown as a cap and only as a cap, so no label or
// line drawn beside one spells a key in its text.
func TestNoKeyNameOutsideACap(t *testing.T) {
	names := keyNames()

	t.Run("when the key names are gathered", func(t *testing.T) {
		t.Run("it should hold every binding's default with more than one letter", func(t *testing.T) {
			assert.Contains(t, names, defaultChords().Bound["close_up"].Label())
		})
	})

	for what, text := range keyedTexts() {
		t.Run("when "+what+" is drawn", func(t *testing.T) {
			t.Run("it should spell no key", func(t *testing.T) {
				assert.Empty(t, spelledKeys(text, names), "%q", text)
			})
		})
	}

	for _, planted := range []string{"esc closes", "Session (space)", "press d again", "shift+n opens it", "1–9 jumps", "3 groups   ↑ ↓ choose, enter focuses it", "c  Start clean"} {
		t.Run("when a label spells a key, as "+planted+" does", func(t *testing.T) {
			t.Run("it should be found", func(t *testing.T) {
				assert.NotEmpty(t, spelledKeys(planted, names))
			})
		})
	}
}
