package render

import (
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
)

// One list vocabulary (item 350): every list panel answers the same keys
// through listKeys, so a key does one thing in every list. The arrows or
// j and k move, tab goes to the next section's first row, 1-9 choose
// that row, enter chooses the focused one, and esc goes back.

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
type listShape struct {
	starts      []int
	typing      bool
	digitsFocus bool
	noDigits    bool
}

// listDigits are the keys that choose rows one to nine.
var listDigits = []ebiten.Key{
	ebiten.KeyDigit1, ebiten.KeyDigit2, ebiten.KeyDigit3, ebiten.KeyDigit4, ebiten.KeyDigit5,
	ebiten.KeyDigit6, ebiten.KeyDigit7, ebiten.KeyDigit8, ebiten.KeyDigit9,
}

// listKeys answers the vocabulary for a list of n rows, moving *focus,
// and says what was asked.
func (g *Game) listKeys(focus *int, n int, shape listShape) listIntent {
	just := g.just
	letters := !shape.typing
	switch {
	case just(ebiten.KeyEscape):
		return listBack
	case just(ebiten.KeyEnter), just(ebiten.KeyKPEnter):
		if n == 0 {
			return listNone
		}
		return listChoose
	case just(ebiten.KeyArrowDown), letters && just(ebiten.KeyJ):
		*focus = max(0, min(*focus+1, n-1))
		return listMoved
	case just(ebiten.KeyArrowUp), letters && just(ebiten.KeyK):
		*focus = max(0, *focus-1)
		return listMoved
	case just(ebiten.KeyTab):
		next := *focus + 1
		if i, ok := slices.BinarySearch(shape.starts, *focus+1); ok || i < len(shape.starts) {
			next = shape.starts[i]
		}
		*focus = max(0, min(next, n-1))
		return listMoved
	}
	if letters && !shape.noDigits {
		for i, key := range listDigits {
			if just(key) && i < n {
				*focus = i
				if shape.digitsFocus {
					return listMoved
				}
				return listChoose
			}
		}
	}
	return listNone
}
