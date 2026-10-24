package render

import (
	"image/color"
	"math"
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
// of texture holding 9.4 MiB of pixels, 7.9 of them the shore's. As its
// own texture it was still 2048² at 4 world units a texel, 16 MiB, so
// the texel grows with the island until the shore fits 1024 a side,
// 4 MiB; Aria could not tell the two apart (2026-10-24, slice 13).
func TestShoreTexture(t *testing.T) {
	for _, tc := range []struct {
		scenario string
		w, h     float64
	}{
		{"a small island", 1000, 1000},
		{"an island 4096 a side", 4096, 4096},
		{"the real history's island", 5856, 5664},
		{"a long, narrow island", 4100, 900},
		{"an island far larger than any yet", 20000, 9000},
	} {
		t.Run("when the shore of "+tc.scenario+" is sized", func(t *testing.T) {
			sea := city.RectAt(-tc.w/2, -tc.h/2, tc.w, tc.h)
			w, h := shoreSize(sea)

			t.Run("it should paint no finer than 4 world units a texel", func(t *testing.T) {
				assert.GreaterOrEqual(t, shoreTexelFor(sea), 4.0)
			})

			t.Run("it should fit 1024 texels a side", func(t *testing.T) {
				assert.LessOrEqual(t, max(w, h), 1024)
			})

			t.Run("and it is drawn", func(t *testing.T) {
				cam := &city.Camera{Zoom: 1.5, Projection: city.Isometric, Heading: 1, Offset: city.Point{X: 300, Y: -40}}
				m := shoreGeoM(cam, sea)
				x, y := m.Apply(float64(w), float64(h))
				texel := shoreTexelFor(sea)
				far := cam.WorldToScreen(city.Point{X: sea.Min.X + float64(w)*texel, Y: sea.Min.Y + float64(h)*texel})

				t.Run("it should put the texture's far corner where the camera puts the sea's", func(t *testing.T) {
					assert.InDelta(t, 0, math.Hypot(x-far.X, y-far.Y), 1e-6)
				})
			})
		})
	}

	t.Run("when the island is at most 4096 on its longer side", func(t *testing.T) {
		t.Run("it should paint at exactly 4 world units a texel", func(t *testing.T) {
			assert.Equal(t, 4.0, shoreTexelFor(city.RectAt(0, 0, 4096, 3000)))
		})
	})

	t.Run("when the shore is made", func(t *testing.T) {
		t.Run("it should be its own texture, not packed into the shared atlas", func(t *testing.T) {
			assert.True(t, shoreOptions().Unmanaged)
		})
	})
}
