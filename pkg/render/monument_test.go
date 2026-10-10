package render

import (
	"fmt"
	"testing"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Bug 45. The sign faces the viewer, so the ground it covers runs
// across the screen and turns with the heading. An inset chosen by eye
// cannot know that, nor about the base's oversail, nor about the
// planting bed — which is how the first cut left the base sitting out
// over the kerb.
//
// This is the guard bug 41 arrived at, applied to a wider object: what
// is asserted is where the thing actually ends up, at every heading,
// not where its anchor is.

const (
	testBed  = 87.0
	testSign = testBed * monumentSpan
)

// covers is every piece of ground the monument puts on the lot.
func covers(cam *city.Camera, at city.Point) []city.Point {
	var out []city.Point
	for _, step := range monumentSteps(ui.MonumentFootprint(testSign), testBed, cam.Zoom) {
		out = append(out, at.Add(screenStep(cam, step)))
	}
	return out
}

func TestMonumentStandsOnItsOwnLot(t *testing.T) {
	lot := city.RectAt(3*city.Tile, 5*city.Tile, 14*city.Tile, 14*city.Tile)

	for _, heading := range []int{0, 90, 180, 270} {
		t.Run(fmt.Sprintf("when the camera faces %d", heading), func(t *testing.T) {
			cam := city.NewCamera()
			cam.Heading = heading
			at, ok := monumentSite(cam, lot, ui.MonumentFootprint(testSign), testBed)
			require.True(t, ok, "the lot should be able to hold a sign")

			t.Run("it should keep every part of the sign on the lot", func(t *testing.T) {
				for _, p := range covers(cam, at) {
					assert.True(t, lot.Contains(p), "%v is off the lot at heading %d", p, heading)
				}
			})

			t.Run("it should leave clear ground between the sign and the kerb", func(t *testing.T) {
				for _, p := range covers(cam, at) {
					assert.LessOrEqual(t, lot.Min.X+kerbClear-p.X, 1e-9)
					assert.LessOrEqual(t, p.X-(lot.Max.X-kerbClear), 1e-9)
				}
			})
		})
	}
}

func TestMonumentPicksTheNearCorner(t *testing.T) {
	lot := city.RectAt(3*city.Tile, 5*city.Tile, 14*city.Tile, 14*city.Tile)

	for _, heading := range []int{0, 90, 180, 270} {
		t.Run(fmt.Sprintf("when the camera faces %d", heading), func(t *testing.T) {
			cam := city.NewCamera()
			cam.Heading = heading
			at, ok := monumentSite(cam, lot, ui.MonumentFootprint(testSign), testBed)
			require.True(t, ok)

			t.Run("it should stand nearer the viewer than the lot's centre", func(t *testing.T) {
				assert.Greater(t, cam.Depth(at), cam.Depth(lot.Center()))
			})
		})
	}
}

func TestMonumentOnATinyLot(t *testing.T) {
	cam := city.NewCamera()

	t.Run("when the lot is too small to hold the sign and its clearance", func(t *testing.T) {
		t.Run("it should say so rather than stand the sign over the kerb", func(t *testing.T) {
			_, ok := monumentSite(cam, city.RectAt(0, 0, city.Tile, city.Tile), ui.MonumentFootprint(testSign), testBed)
			assert.False(t, ok)
		})
	})
}
