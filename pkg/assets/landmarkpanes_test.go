package assets_test

import (
	"fmt"
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/assets"
)

// Item 185: the landmarks light their windows at night, so each must
// carry its panes, and only its glass: the kit's glass is blue, and the
// green of City Hall's cross and the library's awning is not a window.
func TestLandmarkPanes(t *testing.T) {
	atlases, err := assets.LoadKits()
	require.NoError(t, err)
	landmarks := []string{"city-kit-industrial/building-a", "city-kit-commercial/building-l", "city-kit-commercial/building-n"}

	for _, atlas := range atlases {
		pages := map[int]*image.NRGBA{}
		page := func(i int) *image.NRGBA {
			if pages[i] == nil {
				p, err := atlas.DecodePage(i)
				require.NoError(t, err)
				pages[i] = p
			}
			return pages[i]
		}

		for _, name := range landmarks {
			t.Run(fmt.Sprintf("when %s is cut at zoom %g", name, atlas.Zoom), func(t *testing.T) {
				panes := 0
				for _, heading := range assets.Headings {
					sprite, _ := atlas.Sprite(name, heading)
					panes += len(sprite.Panes)
				}

				t.Run("it should have panes from some heading", func(t *testing.T) {
					assert.Positive(t, panes)
				})
			})

			for _, heading := range assets.Headings {
				sprite, _ := atlas.Sprite(name, heading)
				t.Run(fmt.Sprintf("when %s is seen from %d at zoom %g", name, heading, atlas.Zoom), func(t *testing.T) {
					var green []assets.Pane
					for _, pane := range sprite.Panes {
						if !holdsBlueGlass(page(sprite.Page), sprite.Rect.Min, pane) {
							green = append(green, pane)
						}
					}

					t.Run("it should hold glass bluer than green in every pane", func(t *testing.T) {
						assert.Empty(t, green)
					})
				})
			}
		}
	}
}

// blueOverGreen is how much bluer than green the kit's glass is at
// least; the green signs it is told from are greener than blue.
const blueOverGreen = 20

// holdsBlueGlass is whether any pixel inside a pane is the kit's glass.
func holdsBlueGlass(page *image.NRGBA, at image.Point, pane assets.Pane) bool {
	box := image.Rectangle{Min: pane[0], Max: pane[0]}
	for _, c := range pane {
		box = box.Union(image.Rectangle{Min: c, Max: c.Add(image.Pt(1, 1))})
	}
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			if !inPane(pane, float64(x)+0.5, float64(y)+0.5) {
				continue
			}
			c := page.NRGBAAt(at.X+x, at.Y+y)
			if kitGlass(c) && int(c.B)-int(c.G) >= blueOverGreen {
				return true
			}
		}
	}
	return false
}

// kitGlass is the pipeline's rule for the kit's glass.
func kitGlass(c color.NRGBA) bool {
	return c.A > 200 && int(c.B)-int(c.R) > 40 && c.B > 120
}

func inPane(q assets.Pane, x, y float64) bool {
	sign := 0.0
	for i := range q {
		a, b := q[i], q[(i+1)%4]
		cross := float64(b.X-a.X)*(y-float64(a.Y)) - float64(b.Y-a.Y)*(x-float64(a.X))
		if cross == 0 {
			continue
		}
		if sign == 0 {
			sign = cross
		} else if (cross > 0) != (sign > 0) {
			return false
		}
	}
	return true
}
