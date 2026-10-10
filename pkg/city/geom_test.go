package city_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

func TestRectSize(t *testing.T) {
	t.Run("when a rect is measured", func(t *testing.T) {
		r := city.RectAt(10, 20, 30, 40)

		t.Run("it should give its width and height", func(t *testing.T) {
			assert.Equal(t, city.Point{X: 30, Y: 40}, r.Size())
		})
	})
}
