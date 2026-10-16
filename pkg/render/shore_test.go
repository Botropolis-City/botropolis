package render

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// Item 85, slice 3: each kind of ground round the island is painted in
// a colour of its own.
func TestShorePaint(t *testing.T) {
	kinds := []city.Ground{city.GroundLand, city.GroundSand, city.GroundShallows, city.GroundSea, city.GroundFarShore, city.GroundRiver}
	paints := map[color.NRGBA]city.Ground{}
	for _, g := range kinds {
		t.Run("when "+g.String()+" is painted", func(t *testing.T) {
			c := shorePaint(g, 0.5)

			t.Run("it should take a colour no other ground takes", func(t *testing.T) {
				other, taken := paints[c]
				assert.False(t, taken, "%v shares %v with %v", g, c, other)
				paints[c] = g
			})
		})
	}
}
