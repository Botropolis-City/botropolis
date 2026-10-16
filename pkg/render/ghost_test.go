package render

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// Item 85, slice 2: a district being dragged follows the pointer as a
// ghost of its block, over the outline of the slot it would land on.
func TestDistrictGhost(t *testing.T) {
	d := &city.District{Rect: city.RectAt(0, 0, 120, 80)}
	target := city.RectAt(300, 300, 150, 150)

	t.Run("when a district is being dragged", func(t *testing.T) {
		g := districtGhost(city.Drag{District: d, At: city.Point{X: 500, Y: 400}, Target: target, Droppable: true})

		t.Run("it should draw a ghost block of the district's size at the pointer", func(t *testing.T) {
			assert.Equal(t, city.RectAt(440, 360, 120, 80), g.block)
		})

		t.Run("it should outline the target slot", func(t *testing.T) {
			assert.Equal(t, target, g.target)
		})

		t.Run("it should not mark a droppable target refused", func(t *testing.T) {
			assert.False(t, g.refused)
		})
	})

	t.Run("when the target slot is refused", func(t *testing.T) {
		g := districtGhost(city.Drag{District: d, At: city.Point{X: 500, Y: 400}, Target: target})

		t.Run("it should outline it as refused", func(t *testing.T) {
			assert.True(t, g.refused)
		})
	})
}
