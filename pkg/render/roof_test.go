package render

import (
	"testing"

	"botropolis.city/botropolis/pkg/city"
	"github.com/stretchr/testify/assert"
)

// building-a as the atlas ships it: 431 px wide, anchored 314 px above
// its ground point, at an atlas cut for zoom 2. One atlas cell is a
// BuildingSize square — three city tiles — which is the unit these
// numbers are in. Bug 40.
const (
	plantSpriteW  = 431.0
	plantSpriteAY = 314.0
	atlasZoom     = 2.0
)

func TestFootprintSide(t *testing.T) {
	t.Run("when a sprite's width is read off the atlas", func(t *testing.T) {
		t.Run("it should give the footprint's side in world units", func(t *testing.T) {
			assert.InDelta(t, 78.4, footprintSide(plantSpriteW, atlasZoom), 0.1)
		})
	})

	t.Run("when the sprite is one atlas cell wide", func(t *testing.T) {
		t.Run("it should be one BuildingSize square, not one city tile", func(t *testing.T) {
			assert.InDelta(t, city.BuildingSize, footprintSide(264, atlasZoom), 0.001)
		})
	})
}

func TestRoofPerch(t *testing.T) {
	side := footprintSide(plantSpriteW, atlasZoom)
	offset := roofPerch(side)

	t.Run("when the chimney is placed against the roof's own fixtures", func(t *testing.T) {
		t.Run("it should stand east of centre, where Aria asked for it", func(t *testing.T) {
			assert.Greater(t, offset.X, 0.0)
		})

		t.Run("it should stay on the roof rather than hang off its edge", func(t *testing.T) {
			assert.Less(t, offset.X, side/2)
		})
	})

	t.Run("when the camera turns", func(t *testing.T) {
		t.Run("it should be a world offset, so it cannot depend on a heading", func(t *testing.T) {
			assert.Equal(t, roofPerch(side), offset)
		})
	})
}

func TestSunkRows(t *testing.T) {
	// The chimney: 351 px tall, anchored 329 px above its foot, standing
	// on the building's floor with the roof 206.25 px above that foot.
	t.Run("when a chimney rises from inside the building", func(t *testing.T) {
		hidden := sunkRows(206.25, 1, 351, 329)

		t.Run("it should hide the roof's height plus the base below its anchor", func(t *testing.T) {
			assert.InDelta(t, 228.25, hidden, 0.01)
		})

		t.Run("it should leave only the part above the roof showing", func(t *testing.T) {
			assert.InDelta(t, 122.75, 351-hidden, 0.01)
		})
	})

	t.Run("when the piece is drawn at half scale", func(t *testing.T) {
		t.Run("it should count the cut in the atlas's own pixels, not the screen's", func(t *testing.T) {
			assert.InDelta(t, 206.25*2+22, sunkRows(206.25, 0.5, 351, 329), 0.01)
		})
	})
}
