package assets_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/assets"
)

func TestLoadKits(t *testing.T) {
	atlases, err := assets.LoadKits()
	require.NoError(t, err)
	require.NotEmpty(t, atlases)
	one := atlases[0]

	t.Run("when the atlases are loaded", func(t *testing.T) {
		t.Run("it should start at zoom 1", func(t *testing.T) {
			assert.Equal(t, 1.0, one.Zoom)
		})

		t.Run("it should draw a cell 132 px wide at zoom 1", func(t *testing.T) {
			assert.Equal(t, 132.0, one.Tile)
		})

		t.Run("it should scale the tile with the zoom", func(t *testing.T) {
			for _, a := range atlases {
				assert.Equal(t, 132*a.Zoom, a.Tile)
			}
		})

		t.Run("it should cut a flat tile as the map's 2:1 diamond", func(t *testing.T) {
			// The map projects a cell as a diamond twice as wide as it is
			// tall; a sprite cut at any other tilt leaves a sliver at every
			// seam. The straight road is a 1x1 slab 0.02 units thick, so
			// its box is the diamond plus a hair.
			for _, a := range atlases {
				s, ok := a.Sprite("city-kit-roads/road-straight", 0)
				require.True(t, ok)
				over := float64(s.Rect.Dy()) - a.Tile/2
				assert.GreaterOrEqual(t, over, 0.0, "zoom %g: %dx%d is shorter than the diamond", a.Zoom, s.Rect.Dx(), s.Rect.Dy())
				assert.LessOrEqual(t, over, 0.08*a.Tile, "zoom %g: %dx%d is taller than the diamond and its edge", a.Zoom, s.Rect.Dx(), s.Rect.Dy())
			}
		})

		t.Run("it should stand every piece on its own sprite", func(t *testing.T) {
			// The anchor is where the piece's ground point lands inside
			// its sprite. A piece may float above that point (the wires,
			// the drone), so the anchor may sit below the box; it may
			// never sit beside it or above it, because then the map
			// would draw the piece somewhere its point is not — which is
			// how the Space Kit's rover, modelled two tiles off its own
			// origin, came to park in the avenue.
			for _, a := range atlases {
				for _, name := range a.Names() {
					for _, heading := range assets.Headings {
						s, ok := a.Sprite(name, heading)
						require.True(t, ok)
						assert.GreaterOrEqual(t, s.Anchor.X, 0, "zoom %g: %s at %d is anchored left of its sprite", a.Zoom, name, heading)
						assert.LessOrEqual(t, s.Anchor.X, s.Rect.Dx(), "zoom %g: %s at %d is anchored right of its sprite", a.Zoom, name, heading)
						assert.GreaterOrEqual(t, s.Anchor.Y, 0, "zoom %g: %s at %d is anchored above its sprite", a.Zoom, name, heading)
					}
				}
			}
		})

		t.Run("it should decode every page at the manifest's size", func(t *testing.T) {
			for _, page := range one.Pages {
				assert.Equal(t, 2048, page.Bounds().Dx())
			}
		})
	})

	t.Run("when a building is looked up", func(t *testing.T) {
		for _, heading := range assets.Headings {
			t.Run(fmt.Sprintf("it should have the hall at heading %d", heading), func(t *testing.T) {
				s, ok := one.Sprite("city-kit-commercial/building-n", heading)
				require.True(t, ok)
				assert.Positive(t, s.Rect.Dx())
			})
		}

		t.Run("it should keep the anchor inside the sprite", func(t *testing.T) {
			s, _ := one.Sprite("city-kit-commercial/building-n", 0)
			assert.True(t, s.Anchor.X >= 0 && s.Anchor.X <= s.Rect.Dx())
			assert.True(t, s.Anchor.Y >= 0 && s.Anchor.Y <= s.Rect.Dy())
		})

		t.Run("it should put the anchor near the foot", func(t *testing.T) {
			s, _ := one.Sprite("city-kit-commercial/building-n", 0)
			assert.Greater(t, s.Anchor.Y, s.Rect.Dy()/2)
		})

		t.Run("it should wrap a full turn back to the first heading", func(t *testing.T) {
			a, _ := one.Sprite("city-kit-commercial/building-n", 360)
			b, _ := one.Sprite("city-kit-commercial/building-n", 0)
			assert.Equal(t, b, a)
		})

		t.Run("it should know nothing of a piece that is not cut", func(t *testing.T) {
			_, ok := one.Sprite("city-kit-commercial/building-zz", 0)
			assert.False(t, ok)
		})
	})

	t.Run("when the licences are read", func(t *testing.T) {
		for _, kit := range []string{"city-kit-commercial", "city-kit-roads", "city-kit-industrial"} {
			t.Run("it should ship "+kit+"'s CC0 text", func(t *testing.T) {
				text, err := assets.KitLicense(kit)
				require.NoError(t, err)
				assert.Contains(t, text, "Creative Commons Zero")
			})
		}
	})
}
