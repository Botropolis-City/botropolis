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

// Item 134: the train stands beside the cars at the cars' own scale. Real
// container wagons are about 3.0 m wide and 15 m long beside a 1.8 m by
// 4.6 m sedan; the kit's pieces are chunky, so the bands are wide, but
// they are wide around the real ratios rather than around whatever the
// pipeline's scale happens to be.
func TestVehicleScale(t *testing.T) {
	atlases, err := assets.LoadKits()
	require.NoError(t, err)
	require.NotEmpty(t, atlases)
	foot := func(t *testing.T, name string) assets.Foot {
		t.Helper()
		s, ok := atlases[len(atlases)-1].Sprite(name, 0)
		require.True(t, ok, "%s is not in the atlas", name)
		require.Positive(t, s.Foot.Width, "%s has no footprint", name)
		require.Positive(t, s.Foot.Length, "%s has no footprint", name)
		return s.Foot
	}

	t.Run("when a kit piece's footprint is read from the atlas", func(t *testing.T) {
		sedan := foot(t, "car-kit/sedan")
		wagons := []string{"train-kit/train-carriage-container-red", "train-kit/train-carriage-container-blue", "train-kit/train-carriage-container-green"}

		t.Run("and the piece is a train wagon beside the sedan", func(t *testing.T) {
			for _, name := range wagons {
				wagon := foot(t, name)

				t.Run(fmt.Sprintf("it should make %s 1.6 to 2.1 times the sedan's width", name), func(t *testing.T) {
					assert.InDelta(t, 1.85, wagon.Width/sedan.Width, 0.25)
				})

				t.Run(fmt.Sprintf("it should make %s 2.8 to 3.6 times the sedan's length", name), func(t *testing.T) {
					assert.InDelta(t, 3.2, wagon.Length/sedan.Length, 0.4)
				})
			}
		})

		t.Run("and the piece is any locomotive", func(t *testing.T) {
			wagon := foot(t, wagons[0])
			// Item 147: the engines Aria picked, one per window class.
			for _, name := range []string{"train-kit/train-diesel-c", "train-kit/train-electric-bullet-a", "train-kit/train-electric-double-a", "train-kit/train-locomotive-a"} {
				t.Run(fmt.Sprintf("it should make %s at least as wide as a wagon, less 20%%", name), func(t *testing.T) {
					assert.GreaterOrEqual(t, foot(t, name).Width, 0.8*wagon.Width)
				})
			}
		})
	})
}
