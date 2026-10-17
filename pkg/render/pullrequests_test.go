package render

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// Item 89: each pull-request mover is its own model, and the fireworks
// stay over their group.
func TestPullRequestMovers(t *testing.T) {
	t.Run("the delivery truck should be a different model from the depot's packing truck", func(t *testing.T) {
		assert.NotEqual(t, kitTruck, kitDelivery)
	})

	for name, model := range map[string]string{"delivery truck": kitDelivery, "inspector's car": kitInspector} {
		t.Run("the "+name+" should be no model the traffic drives", func(t *testing.T) {
			assert.False(t, slices.Contains(kitCars, model))
		})
	}

	t.Run("when the ribbon is cut", func(t *testing.T) {
		over := city.RectAt(100, 100, 80, 60)
		at := over.Center()

		t.Run("the fireworks should stay over the group", func(t *testing.T) {
			var outside int
			for i := 0; i < 50; i++ {
				for _, p := range sparks(at, over, float64(i)/50) {
					if !over.Contains(p) {
						outside++
					}
				}
			}
			assert.Zero(t, outside)
		})

		t.Run("the fireworks should end by the end of the show", func(t *testing.T) {
			assert.Empty(t, sparks(at, over, 1))
		})
	})
}
