package render

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 99: a landmark's label stood a sprite's height above its foot,
// which is above the sprite's top by all of the sprite below its anchor,
// and so on whatever stands behind it — kiln's sign, in r505.

func landmarkLabelGame(t *testing.T) *Game {
	t.Helper()
	g := kitGame(t)
	g.theme = ui.NewTheme(1)
	f, err := newFaces(g.theme)
	require.NoError(t, err)
	g.faces = f
	return g
}

func isoCamera(heading int, zoom float64) *city.Camera {
	cam := city.NewCamera()
	cam.Projection = city.Isometric
	cam.Heading = heading
	cam.Zoom = zoom
	return cam
}

func TestLandmarkLabelStandsOnItsLandmark(t *testing.T) {
	g := landmarkLabelGame(t)
	lot := plantFixture()
	landmarks := []struct{ name, kit string }{
		{"power plant", kitPlant},
		{"library", kitLibrary},
		{"city hall", kitHall},
	}

	for _, heading := range []int{0, 90, 180, 270} {
		for _, zoom := range []float64{1, 2} {
			cam := isoCamera(heading, zoom)
			for _, l := range landmarks {
				t.Run(fmt.Sprintf("when the %s's label is placed and the camera faces %d at zoom %g", l.name, heading, zoom), func(t *testing.T) {
					sprite := g.landmarkSprite(cam, lot, l.kit)
					require.Greater(t, sprite.Area(), 0.0, "precondition: the sprite is drawn")
					label := g.landmarkLabel(sprite, l.name)

					t.Run("it should anchor within the drawn sprite", func(t *testing.T) {
						assert.True(t, sprite.Contains(label.Anchor), "anchor %v, sprite %v", label.Anchor, sprite)
					})

					t.Run("it should centre its plate over the sprite", func(t *testing.T) {
						assert.InDelta(t, sprite.Center().X, label.Rect.Center().X, 1)
					})

					t.Run("it should rest its plate's bottom within a grid of the sprite's top, without covering it", func(t *testing.T) {
						assert.InDelta(t, sprite.Min.Y-g.theme.Grid()/2, label.Rect.Max.Y, g.theme.Grid()/2)
					})
				})
			}
		}
	}

	t.Run("when the power plant's label is placed", func(t *testing.T) {
		cam := isoCamera(0, 1)
		perch := plantStackPerch(lot, g.kitFootprint(cam, kitPlant))
		_, _, _, _, chimney, ok := g.kitThroughPlace(cam, kitStack, perch, g.kitRoofLift(cam, kitPlant))
		require.True(t, ok, "precondition: the chimney is placed")
		label := g.landmarkLabel(g.landmarkSprite(cam, lot, kitPlant), "power plant")

		t.Run("it should clear the top of the chimney", func(t *testing.T) {
			assert.LessOrEqual(t, label.Rect.Max.Y, chimney.Min.Y)
		})
	})

	t.Run("when the label is placed a sprite's height above the foot, as item 99 found it", func(t *testing.T) {
		cam := isoCamera(0, 1)
		foot := cam.WorldToScreen(lot.Center())
		old := city.Point{X: foot.X, Y: foot.Y - float64(g.kitSize(cam, kitLibrary).Y)}

		t.Run("it should miss the drawn sprite", func(t *testing.T) {
			assert.False(t, g.landmarkSprite(cam, lot, kitLibrary).Contains(old))
		})
	})

	t.Run("when the label is laid out with the map's other labels", func(t *testing.T) {
		label := g.landmarkLabel(city.RectAt(100, 100, 80, 80), "library")

		t.Run("it should be a landmark label, giving way to signs and badges", func(t *testing.T) {
			assert.Equal(t, labelLandmark, label.Kind)
		})
	})
}
