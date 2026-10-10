package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

func TestLayoutButtons(t *testing.T) {
	th := ui.NewTheme(1)

	t.Run("when a row of buttons is laid out", func(t *testing.T) {
		row := ui.LayoutButtons(th, []string{"attach", "stop"}, city.Point{X: 100, Y: 200}, measure7)

		t.Run("it should place one button per label", func(t *testing.T) {
			require.Len(t, row, 2)
		})

		t.Run("it should start at the point", func(t *testing.T) {
			assert.Equal(t, city.Point{X: 100, Y: 200}, row[0].Rect.Min)
		})

		t.Run("it should pad the label one grid unit each side", func(t *testing.T) {
			assert.Equal(t, 6*7+16.0, row[0].Rect.Width())
		})

		t.Run("it should be one line plus a grid unit tall", func(t *testing.T) {
			assert.Equal(t, 16+8.0, row[0].Rect.Height())
		})

		t.Run("it should put the label inside the padding", func(t *testing.T) {
			assert.Equal(t, ui.Text{Text: "attach", At: city.Point{X: 108, Y: 204}, Size: ui.Body}, row[0].Label)
		})

		t.Run("it should leave one grid unit between buttons", func(t *testing.T) {
			assert.Equal(t, row[0].Rect.Max.X+8, row[1].Rect.Min.X)
		})

		t.Run("it should find the button under a point", func(t *testing.T) {
			hit, ok := ui.HitButton(row, row[1].Rect.Center())
			require.True(t, ok)
			assert.Equal(t, "stop", hit.Label.Text)
		})

		t.Run("it should wrap onto a second row when the width runs out", func(t *testing.T) {
			rows := ui.LayoutButtonRows(th, []string{"attach", "stop", "reveal folder"}, city.Point{X: 0, Y: 0}, 140, measure7)
			require.Len(t, rows, 3)
			assert.Equal(t, 0.0, rows[2].Rect.Min.X)
			assert.Equal(t, 16+8+8.0, rows[2].Rect.Min.Y)
		})

		t.Run("it should find nothing beside the row", func(t *testing.T) {
			_, ok := ui.HitButton(row, city.Point{X: 0, Y: 0})
			assert.False(t, ok)
		})
	})
}
