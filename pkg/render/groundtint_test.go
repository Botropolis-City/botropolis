package render

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/plan"
)

// Item 85, slice 4: a tree that means nothing is drawn lower in contrast
// and saturation than anything that can be hovered.
func TestGroundTreeTint(t *testing.T) {
	g := &Game{}

	t.Run("when a ground tree is drawn", func(t *testing.T) {
		tint := g.treeTint(city.Tree{Kind: plan.ParkTree})

		t.Run("it should be drawn through a tint", func(t *testing.T) {
			assert.NotNil(t, tint)
		})

		t.Run("it should let the ground show through, lowering its contrast", func(t *testing.T) {
			assert.Less(t, tint.A(), float32(1))
		})

		t.Run("it should lower its colour, so it is less saturated than the pieces round it", func(t *testing.T) {
			assert.Less(t, tint.G(), tint.A())
		})
	})

	t.Run("when a hoverable piece is drawn", func(t *testing.T) {
		t.Run("it should keep its full colour", func(t *testing.T) {
			assert.Nil(t, g.scenery())
		})
	})

	t.Run("when a street tree is drawn", func(t *testing.T) {
		t.Run("it should keep its full colour, belonging to the road", func(t *testing.T) {
			assert.Nil(t, g.treeTint(city.Tree{Kind: plan.StreetTree}))
		})
	})
}
