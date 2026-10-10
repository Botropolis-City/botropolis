package render

import (
	"testing"

	"botropolis.city/botropolis/pkg/city"
	"github.com/stretchr/testify/assert"
)

// The slab the chimney has to stand on: building-a's own footprint,
// drawn at natural size centred on the plant's reservation.
//
// This helper used to read 431/264 as a count of city tiles, which is
// the unit error bug 39's write-up was retracted for. One atlas cell is
// a BuildingSize square — three city tiles — so the slab is a 78.4-unit
// square, not a 26-unit one, and the reservation around it is 120 x 64.
func slabUnder(plant city.Rect) city.Rect {
	side := footprintSide(plantSpriteW, atlasZoom)
	at := plant.Center()
	return city.RectAt(at.X-side/2, at.Y-side/2, side, side)
}

func plantFixture() city.Rect {
	return city.RectAt(3*city.Tile, 5*city.Tile, city.PlantWidth, city.PlantHeight)
}

func TestPlantStackPerch(t *testing.T) {
	plant := plantFixture()
	side := footprintSide(plantSpriteW, atlasZoom)
	perch := plantStackPerch(plant, side)

	t.Run("when the chimney is placed against the roof's fixtures", func(t *testing.T) {
		t.Run("it should stand on the slab the sprite covers", func(t *testing.T) {
			assert.True(t, slabUnder(plant).Contains(perch))
		})

		t.Run("it should stand east of the point the sprite is drawn at", func(t *testing.T) {
			assert.Greater(t, perch.X, plant.Center().X)
		})

		t.Run("it should stand only a little south of it", func(t *testing.T) {
			assert.Less(t, perch.Y-plant.Center().Y, perch.X-plant.Center().X)
		})
	})

	t.Run("when the footprint's near corner is used instead, as bug 39 did", func(t *testing.T) {
		t.Run("it should fall off the slab, though only just", func(t *testing.T) {
			corner := city.Point{X: plant.Max.X - city.Tile, Y: plant.Max.Y - city.Tile}
			assert.False(t, slabUnder(plant).Contains(corner))
		})
	})
}

func TestStackDepth(t *testing.T) {
	cam := city.NewCamera()
	plant := plantFixture()

	t.Run("when the plant and its chimney are both drawn", func(t *testing.T) {
		t.Run("it should sort the chimney at the building's depth", func(t *testing.T) {
			assert.Equal(t, cam.DepthOf(plant), stackDepth(cam, plant))
		})
	})

	t.Run("when the chimney's own perch is sorted by instead, as bug 39 did", func(t *testing.T) {
		t.Run("it should sort nearer than the slab, which is what floated it", func(t *testing.T) {
			perch := plantStackPerch(plant, footprintSide(plantSpriteW, atlasZoom))
			assert.Greater(t, cam.Depth(perch), stackDepth(cam, plant))
		})
	})
}
