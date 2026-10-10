package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/assets"
	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/plan"
)

func TestTreePiece(t *testing.T) {
	t.Run("when the plan may ask for a species", func(t *testing.T) {
		t.Run("it should have a park piece for every one", func(t *testing.T) {
			assert.Len(t, kitParkTrees, plan.ParkSpecies)
		})

		t.Run("it should have a street piece for every one", func(t *testing.T) {
			assert.Len(t, kitStreetTrees, plan.StreetSpecies)
		})

		t.Run("it should have a bush for every one", func(t *testing.T) {
			assert.Len(t, kitBushes, plan.BushSpecies)
		})
	})

	t.Run("when a planting is a planter", func(t *testing.T) {
		t.Run("it should be the kit's planter whatever its species", func(t *testing.T) {
			assert.Equal(t, kitPlanter, treePiece(city.Tree{Kind: plan.Planter, Variant: 3}))
		})
	})

	t.Run("when a planting names a species the palette does not have", func(t *testing.T) {
		t.Run("it should fall back to the first rather than panic", func(t *testing.T) {
			assert.Equal(t, kitParkTrees[0], treePiece(city.Tree{Kind: plan.ParkTree, Variant: 99}))
		})
	})

	t.Run("when every piece a planting can name is looked up in the atlas", func(t *testing.T) {
		atlases, err := assets.LoadKits()
		require.NoError(t, err)
		require.NotEmpty(t, atlases)
		var every []string
		every = append(every, kitParkTrees...)
		every = append(every, kitStreetTrees...)
		every = append(every, kitBushes...)
		every = append(every, kitPlanter)

		t.Run("it should be a piece the pipeline cut", func(t *testing.T) {
			for _, name := range every {
				_, ok := atlases[0].Sprite(name, 0)
				assert.True(t, ok, "%s is not in the atlas; add it to PIECES in render.py and re-run make sprites", name)
			}
		})
	})
}

func TestSprayFrame(t *testing.T) {
	t.Run("when the clock has just started", func(t *testing.T) {
		t.Run("it should show the first frame", func(t *testing.T) {
			assert.Equal(t, 0, sprayFrame(0, 3))
		})
	})

	t.Run("when a period has passed", func(t *testing.T) {
		t.Run("it should have stepped on one", func(t *testing.T) {
			assert.Equal(t, 1, sprayFrame(SprayPeriod*1.5, 3))
		})
	})

	t.Run("when the cycle has come round", func(t *testing.T) {
		t.Run("it should be back at the first", func(t *testing.T) {
			assert.Equal(t, 0, sprayFrame(SprayPeriod*3, 3))
		})
	})

	t.Run("when the clock stands still, as it does with motion reduced", func(t *testing.T) {
		held := sprayFrame(pulsePeriod/4, 3)

		t.Run("it should hold one frame however long you look", func(t *testing.T) {
			assert.Equal(t, held, sprayFrame(pulsePeriod/4, 3))
		})
	})

	t.Run("when there are no frames to show", func(t *testing.T) {
		t.Run("it should not divide by zero", func(t *testing.T) {
			assert.Equal(t, 0, sprayFrame(12, 0))
		})
	})
}
