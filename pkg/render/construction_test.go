package render

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// Item 89: the fence, scaffolding and crane of a build, as drawn.
func TestSiteLines(t *testing.T) {
	lot := [4]city.Point{{X: 100, Y: 80}, {X: 160, Y: 110}, {X: 100, Y: 140}, {X: 40, Y: 110}}
	roofline := 20.0
	inside := func(p city.Point) bool {
		return p.X >= 40 && p.X <= 160 && p.Y >= 80 && p.Y <= 140
	}

	t.Run("when the scaffolding is up", func(t *testing.T) {
		_, scaffold, _ := siteLines(lot, roofline, city.Site{Scaffold: 1}, 1)

		t.Run("it should draw its poles inside the lot", func(t *testing.T) {
			var outside int
			for _, s := range scaffold {
				if s.pole && !inside(s.a) {
					outside++
				}
			}
			assert.Zero(t, outside)
		})
	})

	t.Run("when the crane is up", func(t *testing.T) {
		_, _, crane := siteLines(lot, roofline, city.Site{Crane: 1}, 1)
		mast := crane[0]

		t.Run("it should stand on the lot", func(t *testing.T) {
			assert.True(t, inside(mast.a))
		})

		t.Run("it should reach above the finished roofline", func(t *testing.T) {
			assert.Less(t, mast.b.Y, roofline)
		})
	})

	t.Run("when the build has finished", func(t *testing.T) {
		fence, scaffold, crane := siteLines(lot, roofline, city.Site{Floors: 1, Lights: 1, Opacity: 1}, 1)

		t.Run("it should draw nothing for the site", func(t *testing.T) {
			assert.Empty(t, append(append(fence, scaffold...), crane...))
		})
	})
}
