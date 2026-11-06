package render

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/ui"
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

	t.Run("when a list filters with /", func(t *testing.T) {
		labels := []string{"kiln reader", "tidepool lead", "kiln sampling", "beacon"}
		frame := func(g *Game, f *listFilter, focus *int, key ebiten.Key, r rune) listIntent {
			g.scripted, g.scriptedRune = key, r
			defer func() { g.scripted, g.scriptedRune = -1, 0 }()
			return g.listKeys(focus, len(labels), listShape{labels: labels, filter: f})
		}
		typed := func(t *testing.T, text string) (*Game, *listFilter, *int) {
			t.Helper()
			g, f, focus := &Game{scripted: -1, clicked: -1}, &listFilter{}, new(int)
			frame(g, f, focus, ebiten.KeySlash, 0)
			for _, r := range text {
				frame(g, f, focus, -1, r)
			}
			return g, f, focus
		}

		t.Run("and / is pressed", func(t *testing.T) {
			_, f, _ := typed(t, "")

			t.Run("it should start a filter", func(t *testing.T) {
				assert.True(t, f.on)
			})
		})

		t.Run("and kiln is typed", func(t *testing.T) {
			_, f, _ := typed(t, "kiln")

			t.Run("it should leave only the rows that hold it", func(t *testing.T) {
				assert.Equal(t, []int{0, 2}, f.visible(labels))
			})
		})

		t.Run("and KILN is typed", func(t *testing.T) {
			_, f, _ := typed(t, "KILN")

			t.Run("it should match whatever the case", func(t *testing.T) {
				assert.Equal(t, []int{0, 2}, f.visible(labels))
			})
		})

		t.Run("and tide is typed", func(t *testing.T) {
			_, _, focus := typed(t, "tide")

			t.Run("it should move the focus onto the row left", func(t *testing.T) {
				assert.Equal(t, 1, *focus)
			})
		})

		t.Run("and kiln is typed, then down is pressed", func(t *testing.T) {
			g, f, focus := typed(t, "kiln")
			frame(g, f, focus, ebiten.KeyArrowDown, 0)

			t.Run("it should move to the next row left, past the ones filtered out", func(t *testing.T) {
				assert.Equal(t, 2, *focus)
			})
		})

		t.Run("and kiln is typed, then j", func(t *testing.T) {
			_, f, _ := typed(t, "kilnj")

			t.Run("it should type j into the filter, not move", func(t *testing.T) {
				assert.Equal(t, "kilnj", string(f.text))
			})
		})

		t.Run("and kiln is typed, then enter is pressed", func(t *testing.T) {
			g, f, focus := typed(t, "kiln")
			frame(g, f, focus, ebiten.KeyArrowDown, 0)
			intent := frame(g, f, focus, ebiten.KeyEnter, 0)

			t.Run("it should choose the focused row left", func(t *testing.T) {
				assert.Equal(t, result{listChoose, 2}, result{intent, *focus})
			})
		})

		t.Run("and backspace is pressed", func(t *testing.T) {
			g, f, focus := typed(t, "kiln")
			frame(g, f, focus, ebiten.KeyBackspace, 0)

			t.Run("it should take the last letter off", func(t *testing.T) {
				assert.Equal(t, "kil", string(f.text))
			})
		})

		t.Run("and esc is pressed", func(t *testing.T) {
			g, f, focus := typed(t, "kiln")
			intent := frame(g, f, focus, ebiten.KeyEscape, 0)

			t.Run("it should clear the filter rather than go back", func(t *testing.T) {
				assert.True(t, !f.on && len(f.text) == 0 && intent != listBack)
			})

			t.Run("and esc is pressed again", func(t *testing.T) {
				intent := frame(g, f, focus, ebiten.KeyEscape, 0)

				t.Run("it should go back", func(t *testing.T) {
					assert.Equal(t, listBack, intent)
				})
			})
		})

		t.Run("and nothing matches, and enter is pressed", func(t *testing.T) {
			g, f, focus := typed(t, "zzz")
			intent := frame(g, f, focus, ebiten.KeyEnter, 0)

			t.Run("it should choose nothing", func(t *testing.T) {
				assert.Equal(t, listNone, intent)
			})

			t.Run("its line should say nothing matches", func(t *testing.T) {
				f.visible(labels)
				assert.Equal(t, "/zzz"+ui.Caret+"   nothing matches", f.shown())
			})
		})

		t.Run("and a row matches", func(t *testing.T) {
			_, f, _ := typed(t, "kil")
			f.visible(labels)

			t.Run("its line should be the filter alone", func(t *testing.T) {
				assert.Equal(t, "/kil"+ui.Caret, f.shown())
			})
		})

		t.Run("and a text field has the focus, and / is pressed", func(t *testing.T) {
			g, f, focus := &Game{scripted: ebiten.KeySlash, clicked: -1}, &listFilter{}, new(int)
			g.listKeys(focus, len(labels), listShape{labels: labels, filter: f, typing: true})

			t.Run("it should leave the slash to the field", func(t *testing.T) {
				assert.False(t, f.on)
			})
		})
	})

	t.Run("when a list renames and removes its rows", func(t *testing.T) {
		pressed := func(key ebiten.Key, opted bool) (renamed, removed []int) {
			g := &Game{scripted: key, clicked: -1}
			focus := 2
			shape := listShape{}
			if opted {
				shape.rename = func(row int) { renamed = append(renamed, row) }
				shape.remove = func(row int) { removed = append(removed, row) }
			}
			g.listKeys(&focus, 5, shape)
			return renamed, removed
		}

		t.Run("and F2 is pressed", func(t *testing.T) {
			renamed, _ := pressed(ebiten.KeyF2, true)

			t.Run("it should rename the focused row", func(t *testing.T) {
				assert.Equal(t, []int{2}, renamed)
			})
		})

		t.Run("and Delete is pressed", func(t *testing.T) {
			_, removed := pressed(ebiten.KeyDelete, true)

			t.Run("it should ask to remove the focused row", func(t *testing.T) {
				assert.Equal(t, []int{2}, removed)
			})
		})

		for name, key := range map[string]ebiten.Key{"F2": ebiten.KeyF2, "Delete": ebiten.KeyDelete} {
			t.Run("and "+name+" is pressed in a list that does not opt in", func(t *testing.T) {
				renamed, removed := pressed(key, false)

				t.Run("it should do nothing", func(t *testing.T) {
					assert.Empty(t, append(renamed, removed...))
				})
			})
		}
	})

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
