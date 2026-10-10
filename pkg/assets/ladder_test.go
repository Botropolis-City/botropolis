package assets_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/assets"
	"botropolis.city/botropolis/pkg/city"
)

// The zoom ladder and the atlas have to agree about how far in the map
// can go, and nothing makes them: the ladder is a list of numbers in
// pkg/city and the atlas is a set of files in pkg/assets.
//
// They disagreed. The ladder went to 4 and the atlas stops at 2, so at
// zoom 3 and 4 there was no sprite and z2 was stretched 1.5x and 2x.
// Every piece blurred; the cooling tower showed it worst only because a
// large smooth curved surface cannot hide upscaling the way a box can,
// which is why it was reported as "the tower is low res" rather than as
// a defect of the whole map.
//
// Cutting the missing levels was budgeted before capping the ladder: z3
// wants 14 pages and z4 wants 23, against a budget of 8 each and z2's 6.
// So the ladder stops where the art does, and this holds it there.
func TestZoomLadderStopsWhereTheAtlasDoes(t *testing.T) {
	atlases, err := assets.LoadKits()
	require.NoError(t, err)
	require.NotEmpty(t, atlases)

	highest := 0.0
	for _, a := range atlases {
		if a.Zoom > highest {
			highest = a.Zoom
		}
	}

	t.Run("when the map is zoomed as far in as it goes", func(t *testing.T) {
		t.Run("it should still have a sprite to draw, rather than stretching one", func(t *testing.T) {
			assert.LessOrEqual(t, city.MaxZoom, highest,
				"the ladder promises zoom %g and the atlas stops at %g", city.MaxZoom, highest)
		})
	})

	t.Run("when the wheel walks its steps", func(t *testing.T) {
		t.Run("it should not step past what the art can serve", func(t *testing.T) {
			for _, step := range city.ZoomSteps {
				require.LessOrEqual(t, step, highest, "step %g is past the atlas", step)
			}
		})

		t.Run("it should end at the top of the ladder", func(t *testing.T) {
			assert.Equal(t, city.MaxZoom, city.ZoomSteps[len(city.ZoomSteps)-1])
		})
	})
}
