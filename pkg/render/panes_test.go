package render

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/assets"
	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// cornerSlack is how far either side of the projected corner a pane may
// reach, in pixels at map scale: the footprint is the model's and the
// corner is the rendered one, with its trim, and they differ by that.
const cornerSlack = 2.0

// Item 166: a ribbon window that wraps a building's near corner is one
// patch of glass in the sprite but a window on each face, and a pane
// across the corner lights and boards the corner itself. The near
// corner is where the building's own footprint comes closest, which
// the scaffold already projects.
func TestPanesStayOnOneFace(t *testing.T) {
	g := kitGame(t)
	for _, atlas := range g.kits.atlases {
		for _, kind := range []city.DistrictType{city.ResidentialDistrict, city.ShoppingDistrict, city.OfficeDistrict, city.IndustrialDistrict} {
			for tier := 1; tier <= state.MaxStoreys; tier++ {
				for _, heading := range assets.Headings {
					cam := &city.Camera{Zoom: atlas.Zoom, Projection: city.Isometric, Heading: heading}
					b := kindBuilding(kind, tier)
					site, ok := g.workFrame(cam, b)
					if !ok {
						continue
					}
					near := site.lot[0]
					for _, p := range site.lot[1:] {
						if p.Y > near.Y {
							near = p
						}
					}
					var spanning [][4]city.Point
					for _, q := range g.facadePanes(cam, buildingPiece(b), g.footOf(cam, b)) {
						if math.Min(q[0].X, q[3].X) < near.X-cornerSlack*atlas.Zoom && math.Max(q[1].X, q[2].X) > near.X+cornerSlack*atlas.Zoom {
							spanning = append(spanning, q)
						}
					}

					t.Run(fmt.Sprintf("when %s is seen from %d at zoom %g", buildingPiece(b), heading, atlas.Zoom), func(t *testing.T) {
						t.Run("it should keep every pane to one face of the building", func(t *testing.T) {
							assert.Empty(t, spanning, "the near corner is at x %.1f", near.X)
						})
					})
				}
			}
		}
	}
}
