package city_test

import (
	"fmt"
	"testing"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
)

// Bug 40. Aria ruled that a session's title is hover-only and a
// project's name keeps a permanent plate — "just put it in a good spot,
// maybe at the bottom". The bottom of a block on screen is its near
// vertex, which is a different world corner at each heading.

func lowestCorner(s *city.Scene, d *city.District) float64 {
	low := s.Camera().WorldToScreen(d.Rect.Min).Y
	for _, p := range []city.Point{
		{X: d.Rect.Max.X, Y: d.Rect.Min.Y},
		d.Rect.Max,
		{X: d.Rect.Min.X, Y: d.Rect.Max.Y},
	} {
		if y := s.Camera().WorldToScreen(p).Y; y > low {
			low = y
		}
	}
	return low
}

func TestDistrictPlateAtTheBottom(t *testing.T) {
	for _, heading := range []int{0, 90, 180, 270} {
		t.Run(fmt.Sprintf("when the isometric camera faces %d", heading), func(t *testing.T) {
			s := scene(t, session("a", cinders, state.Working))
			s.SetProjection(city.Isometric)
			s.Camera().Heading = heading
			d := s.City().Districts[0]

			t.Run("it should sit under the block's near vertex, not its far corner", func(t *testing.T) {
				assert.GreaterOrEqual(t, s.DistrictLabelAt(d, 16, 60).Y, lowestCorner(s, d))
			})
		})
	}
}

func TestDistrictPlateIsPermanent(t *testing.T) {
	t.Run("when a project's sessions are all parked and nothing is hovered", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Parked))
		d := s.City().Districts[0]
		s.Camera().Zoom = 1

		t.Run("it should still carry its plate, because a repo name is what you navigate by", func(t *testing.T) {
			assert.True(t, s.DistrictLabelVisible(d))
		})
	})

	t.Run("when the district is too narrow on screen to read", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Parked))
		d := s.City().Districts[0]
		s.Camera().Zoom = 0.01

		t.Run("it should drop the plate rather than crowd the map", func(t *testing.T) {
			assert.False(t, s.DistrictLabelVisible(d))
		})
	})
}
