package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

func TestLayoutHelp(t *testing.T) {
	th := ui.NewTheme(1)
	keys := []ui.Key{{"drag", "pan"}, {"?", "help"}, {"tab", "next needs-you"}}

	t.Run("when the help is laid out", func(t *testing.T) {
		h := ui.LayoutHelp(th, 800, 600, "Keys", keys, measure7)

		t.Run("it should centre the panel in the window", func(t *testing.T) {
			assert.Equal(t, city.Point{X: 400, Y: 300}, h.Rect.Center())
		})

		t.Run("it should title it at the title size", func(t *testing.T) {
			assert.Equal(t, ui.Text{Text: "Keys", At: h.Rect.Min.Add(city.Point{X: 24, Y: 24}), Size: ui.Title}, h.Title)
		})

		t.Run("it should stack one row per key", func(t *testing.T) {
			require.Len(t, h.Rows, 3)
			assert.Equal(t, 16+8+4.0, h.Rows[1].Chip.Min.Y-h.Rows[0].Chip.Min.Y)
		})

		t.Run("it should align the labels in one column past the widest chip", func(t *testing.T) {
			assert.Equal(t, h.Rows[0].LabelAt.X, h.Rows[2].LabelAt.X)
		})

		t.Run("it should put that column one grid after the widest chip", func(t *testing.T) {
			assert.Equal(t, h.Rect.Min.X+24+4*7+8+8, h.Rows[0].LabelAt.X)
		})

		t.Run("it should size the panel to the longest row plus padding", func(t *testing.T) {
			assert.Equal(t, 24+4*7+8+8+14*7+24.0, h.Rect.Width())
		})
	})
}
