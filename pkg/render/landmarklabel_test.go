package render

import (
	"fmt"
	"math"
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
					foot := cam.WorldToScreen(lot.Center())
					roof := city.Point{X: foot.X, Y: foot.Y - g.kitRoofLiftTurned(cam, l.kit, landmarkTurn)}
					label := g.landmarkLabel(g.landmarkRoof(cam, lot, l.kit), l.name)

					t.Run("it should anchor within the drawn sprite", func(t *testing.T) {
						assert.True(t, sprite.Contains(label.Anchor), "anchor %v, sprite %v", label.Anchor, sprite)
					})

					t.Run("it should rest its plate on the roof", func(t *testing.T) {
						d := label.Rect.Center().Sub(roof)
						assert.LessOrEqual(t, math.Hypot(d.X, d.Y), g.theme.Grid(), "plate %v, roof %v", label.Rect, roof)
					})
				})
			}
		}
	}

	for _, heading := range []int{0, 90, 180, 270} {
		t.Run(fmt.Sprintf("when the power plant's label is placed and the camera faces %d", heading), func(t *testing.T) {
			cam := isoCamera(heading, 1)
			perch := landmarkPerch(lot, g.kitFootprintTurned(cam, kitPlant, landmarkTurn))
			_, _, _, _, chimney, ok := g.kitThroughPlace(cam, kitStack, perch, g.kitRoofLiftTurned(cam, kitPlant, landmarkTurn))
			require.True(t, ok, "precondition: the chimney is placed")
			label := g.landmarkLabel(g.landmarkRoof(cam, lot, kitPlant), "power plant")

			t.Run("it should sit below the chimney's top", func(t *testing.T) {
				assert.Greater(t, label.Rect.Min.Y, chimney.Min.Y)
			})
		})
	}

	t.Run("when the label is placed a sprite's height above the foot, as item 99 found it", func(t *testing.T) {
		cam := isoCamera(0, 1)
		foot := cam.WorldToScreen(lot.Center())
		old := city.Point{X: foot.X, Y: foot.Y - float64(g.kitSizeTurned(cam, kitLibrary, landmarkTurn).Y)}

		t.Run("it should miss the drawn sprite", func(t *testing.T) {
			assert.False(t, g.landmarkSprite(cam, lot, kitLibrary).Contains(old))
		})
	})

	t.Run("when the label is laid out with the map's other labels", func(t *testing.T) {
		label := g.landmarkLabel(city.Point{X: 140, Y: 130}, "library")

		t.Run("it should be a landmark label, giving way to signs and badges", func(t *testing.T) {
			assert.Equal(t, labelLandmark, label.Kind)
		})
	})
}
