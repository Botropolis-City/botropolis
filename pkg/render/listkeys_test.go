package render

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
)

// Item 350, slice 6a: every list answers one vocabulary, from one
// helper: the arrows or j and k move, tab goes to the next section,
// 1-9 choose a row, enter chooses, esc goes back.
func TestListKeys(t *testing.T) {
	type result struct {
		intent listIntent
		focus  int
	}
	press := func(key ebiten.Key, focus, n int, starts []int, typing bool) result {
		g := &Game{scripted: key, clicked: -1}
		intent := g.listKeys(&focus, n, listShape{starts: starts, typing: typing})
		return result{intent, focus}
	}

	t.Run("when 3 is pressed in a list whose enter is the panel's own verb", func(t *testing.T) {
		g := &Game{scripted: ebiten.KeyDigit3, clicked: -1}
		focus := 0
		intent := g.listKeys(&focus, 5, listShape{digitsFocus: true})

		t.Run("it should focus the third row and choose nothing", func(t *testing.T) {
			assert.Equal(t, result{listMoved, 2}, result{intent, focus})
		})
	})

	for _, c := range []struct {
		name   string
		key    ebiten.Key
		focus  int
		n      int
		starts []int
		typing bool
		want   result
	}{
		{"down moves one row down", ebiten.KeyArrowDown, 1, 5, nil, false, result{listMoved, 2}},
		{"j moves one row down", ebiten.KeyJ, 1, 5, nil, false, result{listMoved, 2}},
		{"down at the last row stays there", ebiten.KeyArrowDown, 4, 5, nil, false, result{listMoved, 4}},
		{"up moves one row up", ebiten.KeyArrowUp, 3, 5, nil, false, result{listMoved, 2}},
		{"k moves one row up", ebiten.KeyK, 3, 5, nil, false, result{listMoved, 2}},
		{"up at the first row stays there", ebiten.KeyArrowUp, 0, 5, nil, false, result{listMoved, 0}},
		{"tab in a flat list moves one row down", ebiten.KeyTab, 1, 5, nil, false, result{listMoved, 2}},
		{"tab goes to the next section's first row", ebiten.KeyTab, 1, 6, []int{0, 3}, false, result{listMoved, 3}},
		{"tab in the last section moves one row down", ebiten.KeyTab, 3, 6, []int{0, 3}, false, result{listMoved, 4}},
		{"3 chooses the third row", ebiten.KeyDigit3, 0, 5, nil, false, result{listChoose, 2}},
		{"9 past the last row does nothing", ebiten.KeyDigit9, 1, 5, nil, false, result{listNone, 1}},
		{"enter chooses the focused row", ebiten.KeyEnter, 2, 5, nil, false, result{listChoose, 2}},
		{"enter in an empty list does nothing", ebiten.KeyEnter, 0, 0, nil, false, result{listNone, 0}},
		{"esc goes back", ebiten.KeyEscape, 2, 5, nil, false, result{listBack, 2}},
		{"j while a text field has focus is left to it", ebiten.KeyJ, 1, 5, nil, true, result{listNone, 1}},
		{"3 while a text field has focus is left to it", ebiten.KeyDigit3, 1, 5, nil, true, result{listNone, 1}},
		{"down while a text field has focus still moves", ebiten.KeyArrowDown, 1, 5, nil, true, result{listMoved, 2}},
		{"a key the vocabulary does not hold does nothing", ebiten.KeyQ, 1, 5, nil, false, result{listNone, 1}},
	} {
		t.Run("when "+c.name, func(t *testing.T) {
			t.Run("it should answer as the vocabulary says", func(t *testing.T) {
				assert.Equal(t, c.want, press(c.key, c.focus, c.n, c.starts, c.typing))
			})
		})
	}
}
