package city_test

import (
	"fmt"
	"testing"

	"botropolis.city/botropolis/pkg/city"
	"github.com/stretchr/testify/assert"
)

// Bug 42. A footprint eight units deep was being compared as though it
// were a point at one of its corners, and no single scalar can order a
// point against a box in this projection: keying the box at its back
// corner lets everything behind it draw over it, keying it at its front
// corner lets it draw over everything in front. What does work is a
// pairwise predicate — a is hidden by b when a ends before b begins on
// one of the two axes, in the camera's own frame.

func cell(x, y float64) city.Rect { return city.RectAt(x, y, 1, 1) }

func building() city.Rect {
	return city.Rect{Min: city.Point{X: 5, Y: 10}, Max: city.Point{X: 9, Y: 14}}
}

func TestBehind(t *testing.T) {
	cam := city.NewCamera()

	t.Run("when a tree stands behind a building's back edge", func(t *testing.T) {
		// The probe that proved the bug: depth 16 against the
		// building's keyed 15, so the old sort drew it over the facade.
		t.Run("it should be behind the building", func(t *testing.T) {
			assert.True(t, cam.Behind(cell(7, 9), building()))
		})
	})

	t.Run("when a tree stands off the building's near side", func(t *testing.T) {
		t.Run("it should not be behind the building", func(t *testing.T) {
			assert.False(t, cam.Behind(cell(7, 15), building()))
		})

		t.Run("the building should be behind it, which the front-corner key got wrong", func(t *testing.T) {
			assert.True(t, cam.Behind(building(), cell(7, 15)))
		})
	})

	t.Run("when a tree stands to the building's west, off its x axis", func(t *testing.T) {
		t.Run("it should be behind the building", func(t *testing.T) {
			assert.True(t, cam.Behind(cell(4, 12), building()))
		})
	})

	t.Run("when two footprints overlap on both axes", func(t *testing.T) {
		t.Run("neither should be behind the other, because nothing orders them", func(t *testing.T) {
			assert.False(t, cam.Behind(cell(7, 12), building()) || cam.Behind(building(), cell(7, 12)))
		})
	})

	t.Run("when neighbouring footprints only touch", func(t *testing.T) {
		t.Run("the further one should still be behind, so landmarks keep their tie-break", func(t *testing.T) {
			near := city.Rect{Min: city.Point{X: 9, Y: 10}, Max: city.Point{X: 13, Y: 14}}
			assert.True(t, cam.Behind(building(), near))
		})
	})
}

func TestBehindTurnsWithTheCamera(t *testing.T) {
	for _, tc := range []struct {
		heading int
		behind  bool
		what    string
	}{
		{0, true, "behind the building"},
		{180, false, "in front of it once the city is turned about"},
	} {
		t.Run(fmt.Sprintf("when the camera faces %d", tc.heading), func(t *testing.T) {
			cam := city.NewCamera()
			cam.Heading = tc.heading

			t.Run("it should read the tree as "+tc.what, func(t *testing.T) {
				assert.Equal(t, tc.behind, cam.Behind(cell(7, 9), building()))
			})
		})
	}
}
