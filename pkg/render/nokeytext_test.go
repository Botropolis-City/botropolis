package render

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
)

// fixedKeyNames is the keys the list vocabulary and the camera use,
// which are not in config.Bindings, as text would spell them.
var fixedKeyNames = []string{"esc", "escape", "enter", "return", "tab", "space", "delete", "backspace", "F1", "F2", "↑", "↓", "←", "→", "↑↓", "←→", "j/k", "1-9", "1–9", "+", "-"}

// keyNames is every key a label could spell outside a cap: each
// binding's default as a cap shows it, and the fixed vocabulary. A lone
// letter is left out, since "a" and "i" are words; a letter spelled as
// a key is caught by the press and again patterns instead.
func keyNames() []string {
	names := append([]string(nil), fixedKeyNames...)
	for _, b := range config.Bindings {
		label := defaultChords().Bound[b.Name].Label()
		if len([]rune(label)) > 1 || !isLetter(label) {
			names = append(names, label)
		}
	}
	return names
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
		if isLetter(word) && i > 0 && strings.EqualFold(words[i-1], "press") {
			found = append(found, word)
		}
		if isLetter(word) && i+1 < len(words) && strings.HasPrefix(words[i+1], "again") {
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
	return texts
}

// Item 351: a key is shown as a cap and only as a cap, so no label or
// line drawn beside one spells a key in its text.
func TestNoKeyNameOutsideACap(t *testing.T) {
	names := keyNames()

	t.Run("when the key names are gathered", func(t *testing.T) {
		t.Run("it should hold every binding's default with more than one letter", func(t *testing.T) {
			assert.Contains(t, names, defaultChords().Bound["action_menu"].Label())
		})
	})

	for what, text := range keyedTexts() {
		t.Run("when "+what+" is drawn", func(t *testing.T) {
			t.Run("it should spell no key", func(t *testing.T) {
				assert.Empty(t, spelledKeys(text, names), "%q", text)
			})
		})
	}

	for _, planted := range []string{"esc closes", "Session (space)", "press d again", "shift+n opens it", "1–9 jumps"} {
		t.Run("when a label spells a key, as "+planted+" does", func(t *testing.T) {
			t.Run("it should be found", func(t *testing.T) {
				assert.NotEmpty(t, spelledKeys(planted, names))
			})
		})
	}
}
