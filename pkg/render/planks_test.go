package render

import (
	"fmt"
	"image"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/assets"
	"botropolis.city/botropolis/pkg/city"
)

// Items 109 and 166: a boarded building's planks are nailed to its own
// walls. Every corner of every plank, on every kind at every tier, has
// to land on the building's own pixels, a pixel either way aside: a
// plank that ends in the air is hanging off the building.
func TestPlanksStayOnTheBuilding(t *testing.T) {
	atlases, err := assets.LoadKits()
	require.NoError(t, err)
	for _, atlas := range atlases {
		for _, column := range kinds {
			for _, name := range column {
				for _, heading := range assets.Headings {
					sprite, _ := atlas.Sprite(name, heading)
					page := atlas.Pages[sprite.Page].(*image.NRGBA)
					opaque := func(p city.Point) bool {
						for dy := -1; dy <= 1; dy++ {
							for dx := -1; dx <= 1; dx++ {
								x, y := int(math.Floor(p.X))+dx, int(math.Floor(p.Y))+dy
								if x < 0 || y < 0 || x >= sprite.Rect.Dx() || y >= sprite.Rect.Dy() {
									continue
								}
								if page.NRGBAAt(sprite.Rect.Min.X+x, sprite.Rect.Min.Y+y).A > 0 {
									return true
								}
							}
						}
						return false
					}
					rows := wallSpans(page, sprite)
					var hanging []string
					for i, q := range sprite.Panes {
						var pane [4]city.Point
						for j, c := range q {
							pane[j] = city.Point{X: float64(c.X), Y: float64(c.Y)}
						}
						planks := planksFor(pane, uint32(i)*2654435761)
						nail(planks, rows)
						for _, plank := range planks {
							for _, corner := range plank {
								if !opaque(corner) {
									hanging = append(hanging, fmt.Sprintf("pane %d at (%.1f, %.1f)", i, corner.X, corner.Y))
								}
							}
						}
					}

					t.Run(fmt.Sprintf("when %s is boarded up, seen from %d at zoom %g", name, heading, atlas.Zoom), func(t *testing.T) {
						t.Run("it should nail every plank to the building", func(t *testing.T) {
							assert.Empty(t, hanging)
						})
					})
				}
			}
		}
	}
}
