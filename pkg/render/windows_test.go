package render

import (
	"fmt"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/assets"
	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 166, with item 155's storeys: which of a building's windows are
// lit tonight.
func TestWindowLights(t *testing.T) {
	g := kitGame(t)
	cam := city.NewCamera()
	standing := func(b *city.Building) bool {
		_, ok := g.windowLights(cam, b, 1, 1)
		return ok
	}

	t.Run("when a lived-in building stands at night", func(t *testing.T) {
		b := kindBuilding(city.OfficeDistrict, 4)
		b.Lit = true
		lights, ok := g.windowLights(cam, b, 1, 1)

		t.Run("it should light its windows", func(t *testing.T) {
			assert.True(t, ok)
		})

		t.Run("it should light every one of its panes", func(t *testing.T) {
			assert.True(t, lights.all)
		})
	})

	for _, c := range []struct {
		name  string
		build func() *city.Building
	}{
		{"boarded up", func() *city.Building {
			b := kindBuilding(city.OfficeDistrict, 4)
			b.Lit, b.Blinds, b.BoardedUp, b.Rung = true, true, true, state.RungShuttered
			return b
		}},
		{"resting behind its blinds", func() *city.Building {
			b := kindBuilding(city.OfficeDistrict, 4)
			b.Lit, b.Blinds, b.Rung = true, true, state.RungResting
			return b
		}},
		{"ruins", func() *city.Building {
			b := kindBuilding(city.OfficeDistrict, 4)
			b.Lit, b.Session.ConversationGone = true, true
			return b
		}},
		{"not live", func() *city.Building {
			return kindBuilding(city.OfficeDistrict, 4)
		}},
	} {
		t.Run("when a building is "+c.name, func(t *testing.T) {
			t.Run("it should light nothing", func(t *testing.T) {
				assert.False(t, standing(c.build()))
			})
		})
	}

	t.Run("when a building's lights are off, on the tug or the truck", func(t *testing.T) {
		b := kindBuilding(city.OfficeDistrict, 4)
		b.Lit = true

		t.Run("it should light nothing", func(t *testing.T) {
			_, ok := g.windowLights(cam, b, 1, 0)
			assert.False(t, ok)
		})
	})

	t.Run("when a building's lights are fading in", func(t *testing.T) {
		b := kindBuilding(city.OfficeDistrict, 4)
		b.Lit = true

		t.Run("it should draw them at that strength", func(t *testing.T) {
			lights, _ := g.windowLights(cam, b, 1, 0.4)
			assert.InDelta(t, 0.4, lights.alpha, 1e-9)
		})
	})

	t.Run("when a four-storey building is two and a half storeys built", func(t *testing.T) {
		b := kindBuilding(city.OfficeDistrict, 4)
		b.Lit = true
		size := g.kitSize(cam, buildingPiece(b))
		storeys, storey, plinth := g.storeyFrame(cam, b, size)
		require.Equal(t, 4, storeys)
		built := plinth + 2.5*storey
		lights, ok := g.windowLights(cam, b, built/(plinth+float64(storeys)*storey), 1)
		require.True(t, ok)
		atlas := g.kits.pick(cam.Zoom)
		sprite, _ := atlas.Sprite(buildingPiece(b), cam.Heading)
		heights := paneHeights(sprite, atlas.Zoom, cam.Heading)
		of := paneStoreys(sprite, atlas.Zoom, cam.Heading)
		above := 0
		for i := range sprite.Panes {
			if of[i] < lights.storeys && heights[i][1] > built {
				above++
			}
		}

		t.Run("it should light its two finished storeys", func(t *testing.T) {
			assert.Equal(t, 2, lights.storeys)
		})

		t.Run("it should leave the storey under construction, and above, dark", func(t *testing.T) {
			assert.False(t, lights.all)
		})

		t.Run("it should light no pane reaching above the built height", func(t *testing.T) {
			assert.Zero(t, above)
		})
	})
}

// Item 166: a storey is a sloped band across each face, so which storey
// a pane is on is measured from the ground of its own face. Measured
// right, every pane sits inside one storey, and a building of n storeys
// has windows on each of them.
func TestPaneStoreys(t *testing.T) {
	atlases, err := assets.LoadKitManifests()
	require.NoError(t, err)
	for _, atlas := range atlases {
		storey := storeyHeight(atlas.Zoom)
		plinth := kindPlinth / kindStorey * storey
		slack := 1.5 * atlas.Zoom
		for kind, column := range kinds {
			for tier, name := range column {
				for _, heading := range assets.Headings {
					sprite, _ := atlas.Sprite(name, heading)
					heights := paneHeights(sprite, atlas.Zoom, heading)
					of := paneStoreys(sprite, atlas.Zoom, heading)
					var astride []string
					seen := map[int]bool{}
					for i, h := range heights {
						seen[of[i]] = true
						floor := plinth + float64(of[i])*storey
						if of[i] == 0 {
							floor = 0
						}
						if h[0] < floor-slack || h[1] > plinth+float64(of[i]+1)*storey+slack {
							astride = append(astride, fmt.Sprintf("pane %d at %.1f..%.1f", i, h[0], h[1]))
						}
					}

					t.Run(fmt.Sprintf("when %s is seen from %d at zoom %g", name, heading, atlas.Zoom), func(t *testing.T) {
						t.Run("it should stand every pane within one storey", func(t *testing.T) {
							assert.Empty(t, astride, "storeys %.1f px over a %.1f px plinth", storey, plinth)
						})

						t.Run(fmt.Sprintf("it should have windows on each of its %d storeys", tier+1), func(t *testing.T) {
							assert.Len(t, seen, tier+1, "%s", kind)
						})
					})
				}
			}
		}
	}
}

// Item 166: the discs were three draws a storey; the windows are one
// draw a building, whatever its size. The manager's bound: no more
// window draws than there are lit buildings.
func TestNightLightsCost(t *testing.T) {
	g := kitGame(t)
	g.scene = city.NewScene(nil)
	cam := city.NewCamera()
	var buildings []*city.Building
	for _, kind := range []city.DistrictType{city.ResidentialDistrict, city.ShoppingDistrict, city.OfficeDistrict, city.IndustrialDistrict} {
		for tier := 1; tier <= state.MaxStoreys; tier++ {
			b := kindBuilding(kind, tier)
			b.Session.ID = fmt.Sprintf("%s-%d", kind, tier)
			b.Lit = true
			buildings = append(buildings, b)
		}
	}
	g.beginLights(64, 64)
	for _, b := range buildings {
		g.lightBuilding(cam, b)
	}
	g.endLights(ebiten.NewImage(64, 64))

	t.Run("when twenty lit buildings of every kind and tier are drawn at night", func(t *testing.T) {
		t.Run("it should draw each one's windows in a single call", func(t *testing.T) {
			assert.Equal(t, len(buildings), g.windowDraws)
		})
	})
}
