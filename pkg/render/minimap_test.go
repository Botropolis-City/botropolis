package render

import (
	"image/color"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
)

// Item 270: the minimap drew a box for every district and a dot for
// every building each frame, 3,300 of them on 900 districts, though
// they change only with the city, the minimap's box and the heading.
func TestMinimapCache(t *testing.T) {
	box := city.RectAt(1240, 600, 300, 220)
	other := city.RectAt(1200, 560, 340, 260)
	t.Run("when the minimap of a city of 200 districts, zoomed in, is drawn on two frames in a row", func(t *testing.T) {
		d := manyDistricts(t)
		g := d.game
		img := ebiten.NewImage(1600, 900)
		before := g.minimapPaints
		g.minimap(img, box)
		first := g.minimapPaints - before
		g.minimap(img, box)
		second := g.minimapPaints - before - first

		t.Run("it should paint the map on the first frame", func(t *testing.T) {
			assert.Equal(t, 1, first)
		})

		t.Run("it should paint none of it on the second", func(t *testing.T) {
			assert.Zero(t, second)
		})

		t.Run("it should keep the boxes a minimap painted afresh draws, moved by whole pixels", func(t *testing.T) {
			type drawn struct {
				x, y, w, h float32
				c          color.Color
			}
			var boxes []drawn
			g.minimapPainter = func(_ *ebiten.Image, x, y, w, h float32, c color.Color) { boxes = append(boxes, drawn{x, y, w, h, c}) }
			defer func() { g.minimapPainter = nil }()
			g.noStill = true
			g.minimap(img, box)
			g.noStill = false
			fresh := boxes
			boxes = nil
			g.mini.img = nil
			g.minimap(img, box)
			at := g.mini.at
			for i := range boxes {
				boxes[i].x += float32(at.X)
				boxes[i].y += float32(at.Y)
			}
			assert.Equal(t, fresh, boxes)
		})

		t.Run("and the camera pans", func(t *testing.T) {
			before := g.minimapPaints
			g.scene.Camera().Pan(city.Point{X: 40, Y: 25})
			g.minimap(img, box)

			t.Run("it should paint none of the map again", func(t *testing.T) {
				assert.Equal(t, before, g.minimapPaints)
			})
		})

		t.Run("and the camera turns to another heading", func(t *testing.T) {
			before := g.minimapPaints
			g.scene.Camera().Turn(1, g.scene.City().Bounds().Center())
			g.minimap(img, box)

			t.Run("it should paint the map again", func(t *testing.T) {
				assert.Equal(t, before+1, g.minimapPaints)
			})
		})

		t.Run("and the minimap's box changes, as a resized window moves it", func(t *testing.T) {
			before := g.minimapPaints
			g.minimap(img, other)

			t.Run("it should paint the map again", func(t *testing.T) {
				assert.Equal(t, before+1, g.minimapPaints)
			})
		})

		t.Run("and a new snapshot rebuilds the city", func(t *testing.T) {
			was := g.scene.City()
			require.NoError(t, d.Feed(d.game.home))
			require.NoError(t, d.Run(2))
			require.NotSame(t, was, g.scene.City(), "the snapshot did not rebuild the city")
			before := g.minimapPaints
			g.minimap(img, other)

			t.Run("it should paint the map again", func(t *testing.T) {
				assert.Equal(t, before+1, g.minimapPaints)
			})
		})
	})
}
