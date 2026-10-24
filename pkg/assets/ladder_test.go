package assets_test

import (
	"math"
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
//
// Item 162, slice 14: "where the art does" is now a draw-up, not a cut.
// z2 gave way to z1.5 drawn up to 2.0, 1.33x, which Aria judged against
// z2 on 2026-10-24 and could not tell apart, so the assertion that the
// highest cut reaches MaxZoom was retired; what holds the ladder now is
// that no step draws a sprite up by KitDrawUpMost or more, which a ladder
// running to 3 or 4 still fails.
func TestZoomLadderStopsWhereTheAtlasDoes(t *testing.T) {
	atlases, err := assets.LoadKits()
	require.NoError(t, err)
	require.NotEmpty(t, atlases)

	t.Run("when the wheel walks its steps", func(t *testing.T) {
		t.Run("it should never draw a sprite up by as much as KitDrawUpMost", func(t *testing.T) {
			for _, step := range city.ZoomSteps {
				least := math.Inf(1)
				for _, a := range atlases {
					least = math.Min(least, step/a.Zoom)
				}
				require.Less(t, least, assets.KitDrawUpMost, "step %g draws every level up by %gx or more", step, least)
			}
		})

		t.Run("it should end at the top of the ladder", func(t *testing.T) {
			assert.Equal(t, city.MaxZoom, city.ZoomSteps[len(city.ZoomSteps)-1])
		})
	})
}
