package render

import (
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// One list vocabulary (item 350): every list panel answers the same keys
// through listKeys, so a key does one thing in every list. The arrows or
// j and k move, tab goes to the next section's first row, 1-9 choose
// that row, enter chooses the focused one, esc goes back, and / filters
// the rows by what is typed.

// listIntent is what a key asked of a list.
type listIntent int

const (
	listNone listIntent = iota
	listMoved
	listChoose
	listBack
)

// listShape is what a list is made of beyond its length: where each of
// its sections starts, and whether a text field in it has the focus,
// which takes letters and digits first.
//
// digitsFocus is a list whose Enter is the panel's own verb rather than
// a row's, the tidy-up walk's and close-up's: there 1-9 only focus a row.
//
// noDigits is a list docked on the map, the dive's, where 1-6 stay the
// lenses' keys.
//
// labels and filter, when the list has them, are what each row is
// called and the filter / types into.
//
// rename and remove, in a list whose rows can be renamed or removed, are
// what F2 and Delete do to the focused row; remove asks first, through
// the one dialog for destroying things, whose own Delete confirms.
type listShape struct {
	starts      []int
	typing      bool
	digitsFocus bool
	noDigits    bool
	labels      []string
	filter      *listFilter
	rename      func(row int)
	remove      func(row int)
}

// listFilter is the text a list is narrowed by while / is on.
type listFilter struct {
	on   bool
	text []rune
}

// visible is the rows a filter leaves, by index: every row whose label
// holds its text, whatever the case.
func (f *listFilter) visible(labels []string) []int {
	var rows []int
	on := f != nil && f.on
	want := ""
	if on {
		want = strings.ToLower(string(f.text))
	}
	for i, label := range labels {
		if !on || strings.Contains(strings.ToLower(label), want) {
			rows = append(rows, i)
		}
	}
	return rows
}

// shown is the filter as its line reads, or "" when it is off.
func (f *listFilter) shown() string {
	if f == nil || !f.on {
		return ""
	}
	return "/" + string(f.text)
}

// left is the rows a filter leaves, as a set, for a panel drawing its
// rows to skip the rest.
func (f *listFilter) left(labels []string) map[int]bool {
	set := map[int]bool{}
	for _, i := range f.visible(labels) {
		set[i] = true
	}
	return set
}

// drawFilter draws a list's filter line at a point, in the accent, and
// says how much height it took: none while the filter is off.
func (g *Game) drawFilter(screen *ebiten.Image, f *listFilter, at city.Point, size ui.Size) float64 {
	line := f.shown()
	if line == "" {
		return 0
	}
	_, h := g.faces.Measure(line, size)
	g.text(screen, at, line+"▏", size, g.theme.Palette.Accent)
	return h + g.theme.Grid()/2
}

// listDigits are the keys that choose rows one to nine.
var listDigits = []ebiten.Key{
	ebiten.KeyDigit1, ebiten.KeyDigit2, ebiten.KeyDigit3, ebiten.KeyDigit4, ebiten.KeyDigit5,
	ebiten.KeyDigit6, ebiten.KeyDigit7, ebiten.KeyDigit8, ebiten.KeyDigit9,
}

// listKeys answers the vocabulary for a list of n rows, moving *focus,
// which is a row's index in the whole list, and says what was asked.
func (g *Game) listKeys(focus *int, n int, shape listShape) listIntent {
	just := g.just
	f := shape.filter
	rows := make([]int, n)
	for i := range rows {
		rows[i] = i
	}
	took := false
	if f != nil {
		if f.on {
			took = g.filterKeys(f)
		} else if !shape.typing && just(ebiten.KeySlash) && !g.shifted() {
			f.on, took = true, true
			g.filterTyping = true
		}
		if len(shape.labels) == n {
			rows = f.visible(shape.labels)
		}
		// Typed letters reach the filter while it is on; answerKeys
		// clears this each frame, so it ends with the panel.
		g.filterTyping = f.on
	}
	// The focus stays on a row the filter leaves. A list with nothing
	// chosen yet, the dive's before its first step, stays so.
	at := slices.Index(rows, *focus)
	if at < 0 && *focus >= 0 && len(rows) > 0 {
		at, *focus = 0, rows[0]
	}
	if took {
		return listMoved
	}
	letters := !shape.typing && (f == nil || !f.on)
	to := func(i int) {
		if len(rows) > 0 {
			*focus = rows[max(0, min(i, len(rows)-1))]
		}
	}
	switch {
	case just(ebiten.KeyEscape):
		return listBack
	case just(ebiten.KeyEnter), just(ebiten.KeyKPEnter):
		if len(rows) == 0 {
			return listNone
		}
		return listChoose
	case just(ebiten.KeyArrowDown), letters && just(ebiten.KeyJ):
		to(at + 1)
		return listMoved
	case just(ebiten.KeyArrowUp), letters && just(ebiten.KeyK):
		to(at - 1)
		return listMoved
	case just(ebiten.KeyF2) && shape.rename != nil && at >= 0:
		shape.rename(*focus)
		return listMoved
	case just(ebiten.KeyDelete) && shape.remove != nil && at >= 0:
		shape.remove(*focus)
		return listMoved
	case just(ebiten.KeyTab):
		next := at + 1
		for i, row := range rows {
			if i > at && slices.Contains(shape.starts, row) {
				next = i
				break
			}
		}
		to(next)
		return listMoved
	}
	if letters && !shape.noDigits {
		for i, key := range listDigits {
			if just(key) && i < len(rows) {
				*focus = rows[i]
				if shape.digitsFocus {
					return listMoved
				}
				return listChoose
			}
		}
	}
	return listNone
}

// filterKeys answers a filter that is on: esc clears it, backspace takes
// a letter off, and what is typed goes into it. It reports whether it
// took the key.
func (g *Game) filterKeys(f *listFilter) bool {
	switch {
	case g.just(ebiten.KeyEscape):
		f.on, f.text = false, nil
		g.filterTyping = false
		return true
	case g.just(ebiten.KeyBackspace):
		if len(f.text) > 0 {
			f.text = f.text[:len(f.text)-1]
		}
		return true
	}
	typed := false
	if g.scriptedRune != 0 {
		f.text = append(f.text, g.scriptedRune)
		typed = true
	}
	if !g.driven && g.scripted < 0 {
		for _, r := range ebiten.AppendInputChars(nil) {
			f.text = append(f.text, r)
			typed = true
		}
	}
	return typed
}
