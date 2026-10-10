package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

func TestLayoutPlate(t *testing.T) {
	th := ui.NewTheme(1)

	t.Run("when a plate is laid out at a point", func(t *testing.T) {
		plate := ui.LayoutPlate(th, "mullet", city.Point{X: 100, Y: 50}, ui.Small, measure7)

		t.Run("it should pad the text half a grid each side", func(t *testing.T) {
			assert.Equal(t, 6*7+8.0, plate.Rect.Width())
		})

		t.Run("it should be one line tall plus padding", func(t *testing.T) {
			assert.Equal(t, 16+4.0, plate.Rect.Height())
		})

		t.Run("it should start where it was asked", func(t *testing.T) {
			assert.Equal(t, city.Point{X: 100, Y: 50}, plate.Rect.Min)
		})

		t.Run("it should place the text inside the padding", func(t *testing.T) {
			assert.Equal(t, city.Point{X: 104, Y: 52}, plate.TextAt)
		})
	})
}
