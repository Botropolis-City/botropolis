package ui_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/ui"
)

// Item 350, slice 5b: the settings' Keys panel lists every binding under
// where it answers, each with what it does and its key as a cap, and
// scrolls to keep the focused row drawn.
func TestLayoutKeysPanel(t *testing.T) {
	th := ui.NewTheme(1)
	sections := []string{"At the map", "In the action menu", "In the finder"}
	rows := func(n int) []ui.KeyRow {
		var out []ui.KeyRow
		for i := range n {
			out = append(out, ui.KeyRow{Section: sections[min(i/(n/3+1), 2)], Label: fmt.Sprintf("binding %d", i), Key: "k"})
		}
		return out
	}
	laid := func(rs []ui.KeyRow, focus int, height float64) ui.KeysPanel {
		return ui.LayoutKeysPanel(th, 1280, height, rs, focus, measure7)
	}

	t.Run("when nine rows are laid out in a tall window", func(t *testing.T) {
		p := laid(rows(9), 0, 2000)

		t.Run("it should head the sections in order", func(t *testing.T) {
			var heads []string
			for _, h := range p.Headings {
				heads = append(heads, h.Text)
			}
			assert.Equal(t, sections, heads)
		})

		t.Run("it should lay one row per binding", func(t *testing.T) {
			assert.Len(t, p.Rows, 9)
		})

		t.Run("it should put each row under its own heading", func(t *testing.T) {
			assert.True(t, p.Rows[0].Label.At.Y > p.Headings[0].At.Y && p.Rows[8].Label.At.Y > p.Headings[2].At.Y)
		})

		t.Run("it should give each row its label and its key as a cap", func(t *testing.T) {
			assert.Equal(t, []string{"binding 4", "k"}, []string{p.Rows[4].Label.Text, p.Rows[4].Cap.Label.Text})
		})

		t.Run("it should mark the focused row", func(t *testing.T) {
			assert.True(t, p.Rows[0].Focused && !p.Rows[1].Focused)
		})
	})

	t.Run("when the focus is past the window's room", func(t *testing.T) {
		all := rows(60)
		p := laid(all, 59, 600)
		require.NotEmpty(t, p.Rows)

		t.Run("it should draw the focused row", func(t *testing.T) {
			assert.Equal(t, 59, p.Rows[len(p.Rows)-1].Index)
		})

		t.Run("it should leave the first rows out", func(t *testing.T) {
			assert.Greater(t, p.Rows[0].Index, 0)
		})

		t.Run("it should keep inside the window", func(t *testing.T) {
			assert.LessOrEqual(t, p.Rect.Max.Y, 600.0)
		})
	})

	t.Run("when a row is recording", func(t *testing.T) {
		rs := rows(9)
		rs[2].Recording = true
		p := laid(rs, 2, 2000)

		t.Run("its cap should ask for a key", func(t *testing.T) {
			assert.Equal(t, "press a key…", p.Rows[2].Cap.Label.Text)
		})
	})

	t.Run("when a row's key was refused", func(t *testing.T) {
		rs := rows(9)
		rs[2].Refused = "timeline and breakdown are both x"
		p := laid(rs, 2, 2000)

		t.Run("it should carry the reason on the row's own line", func(t *testing.T) {
			assert.Equal(t, rs[2].Refused, p.Rows[2].Note.Text)
		})

		t.Run("it should put the reason under the row, above the next", func(t *testing.T) {
			assert.True(t, p.Rows[2].Note.At.Y > p.Rows[2].Label.At.Y && p.Rows[2].Note.At.Y < p.Rows[3].Label.At.Y)
		})
	})
}
