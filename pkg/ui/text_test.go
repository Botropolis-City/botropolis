package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 102: a plate the label layout stepped aside keeps its text where
// it was on the plate.
func TestPlateMoved(t *testing.T) {
	p := ui.Plate{Rect: city.RectAt(10, 20, 40, 12), TextAt: city.Point{X: 14, Y: 23}, Size: ui.Small}

	t.Run("when a plate is moved to a new corner", func(t *testing.T) {
		moved := p.Moved(city.Point{X: 100, Y: 50})

		t.Run("it should keep its size, its text's place on it and its size of type", func(t *testing.T) {
			assert.Equal(t, ui.Plate{Rect: city.RectAt(100, 50, 40, 12), TextAt: city.Point{X: 104, Y: 53}, Size: ui.Small}, moved)
		})
	})
}
