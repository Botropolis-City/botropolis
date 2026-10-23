package render

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// Item 85, slice 3: each kind of ground round the island is painted in
// a colour of its own.
// The top-down view has its own darker map palette; the island under it
// has to share that palette, or the plan reads as a rectangle on it.
func TestShorePaletteFollowsTheProjection(t *testing.T) {
	for _, tc := range []struct {
		scenario    string
		flat, night bool
		land, river color.NRGBA
	}{
		{"by day in the isometric view", false, false, colorKitGrass, colorKitWater},
		{"by day in the top-down view", true, false, colorGround, colorWater},
		{"by night in the top-down view", true, true, colorGroundNight, colorWaterNight},
	} {
		t.Run("when the island is painted "+tc.scenario, func(t *testing.T) {
			p := shorePaletteFor(tc.flat, tc.night)

			t.Run("it should paint the land in that view's ground colour", func(t *testing.T) {
				assert.Equal(t, tc.land, p.paint(city.GroundLand, 0))
			})

			t.Run("it should paint the river in that view's water colour", func(t *testing.T) {
				assert.Equal(t, tc.river, p.paint(city.GroundRiver, 0))
			})
		})
	}
}

func TestShorePaint(t *testing.T) {
	kinds := []city.Ground{city.GroundLand, city.GroundSand, city.GroundShallows, city.GroundSea, city.GroundFarShore, city.GroundRiver}
	paints := map[color.NRGBA]city.Ground{}
	for _, g := range kinds {
		t.Run("when "+g.String()+" is painted", func(t *testing.T) {
			c := shorePaint(g, 0.5)

			t.Run("it should take a colour no other ground takes", func(t *testing.T) {
				other, taken := paints[c]
				assert.False(t, taken, "%v shares %v with %v", g, c, other)
				paints[c] = g
			})
		})
	}
}

// Item 162: the shore was made on Ebitengine's shared atlas, and on the
// real history it is 1464×1416, which grew the atlas to 4096×2048: 32 MiB
// of texture holding 9.4 MiB of pixels, 7.9 of them the shore's.
func TestShoreTexture(t *testing.T) {
	t.Run("when the shore is made", func(t *testing.T) {
		t.Run("it should be its own texture, not packed into the shared atlas", func(t *testing.T) {
			assert.True(t, shoreOptions().Unmanaged)
		})
	})
}
