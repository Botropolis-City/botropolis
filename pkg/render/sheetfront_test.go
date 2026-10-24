package render

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// Item 233: the night check runs at every wheel step, and a thing that
// stands in front of a lit tower only when its words are legible is
// left off below ui.MinSignPx by its own layout, the one the drawing
// code uses, not by a list of zooms.
func TestLightFixturesFollowTheirLayouts(t *testing.T) {
	g := bakeGame(t)

	t.Run("when the tower is drawn at the farthest wheel step", func(t *testing.T) {
		unplaced := g.unplacedInFront(city.ZoomSteps[0])

		t.Run("it should leave off the district sign and its own painted sign", func(t *testing.T) {
			assert.ElementsMatch(t, []string{"a district sign", "its own painted sign"}, unplaced)
		})
	})

	t.Run("when the tower is drawn at the closest wheel step", func(t *testing.T) {
		unplaced := g.unplacedInFront(city.MaxZoom)

		t.Run("it should leave off nothing", func(t *testing.T) {
			assert.Empty(t, unplaced)
		})
	})
}
