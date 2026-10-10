package city_test

import (
	"testing"

	"botropolis.city/botropolis/pkg/city"
	"github.com/stretchr/testify/assert"
)

func TestProjection(t *testing.T) {
	t.Run("when the projection is isometric", func(t *testing.T) {
		iso := city.Isometric

		t.Run("it should take a world point there and back", func(t *testing.T) {
			w := city.Point{X: 123, Y: 45}
			back := iso.Invert(iso.Apply(w))
			assert.InDelta(t, w.X, back.X, 1e-9)
			assert.InDelta(t, w.Y, back.Y, 1e-9)
		})

		t.Run("it should project a building footprint to a native-width diamond", func(t *testing.T) {
			b := iso.Bounds(city.RectAt(0, 0, city.BuildingSize, city.BuildingSize))
			assert.InDelta(t, city.IsoTileWidth, b.Width(), 1e-9)
			assert.InDelta(t, city.IsoTileWidth/2, b.Height(), 1e-9)
		})

		t.Run("it should put the world origin at the diamond's top corner", func(t *testing.T) {
			c := iso.Corners(city.RectAt(0, 0, 10, 10))
			assert.Equal(t, city.Point{}, c[0])
			assert.Less(t, c[0].Y, c[2].Y)
		})
	})

	t.Run("when the projection is top-down", func(t *testing.T) {
		t.Run("it should change nothing", func(t *testing.T) {
			assert.Equal(t, city.Point{X: 3, Y: 4}, city.TopDown.Apply(city.Point{X: 3, Y: 4}))
		})
	})

	t.Run("when a projection is parsed from config", func(t *testing.T) {
		cases := map[string]city.Projection{"iso": city.Isometric, "isometric": city.Isometric, "top": city.TopDown}
		for in, want := range cases {
			t.Run("and the word is "+in, func(t *testing.T) {
				got, ok := city.ParseProjection(in)
				assert.True(t, ok)
				assert.Equal(t, want, got)
			})
		}

		t.Run("and the word is unknown", func(t *testing.T) {
			_, ok := city.ParseProjection("cabinet")
			assert.False(t, ok)
		})
	})
}
