package render

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 132: the rail stays on the ground and the road goes over it, so
// a train under the first span draws beneath the deck and a truck on
// the deck draws over it, at every heading and whatever order they were
// gathered in.
func TestBridgeDrawOrder(t *testing.T) {
	start := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	snap := state.Snapshot{At: start, Sessions: []state.Session{{ID: "a", CWD: "/p/a", State: state.Working, StartedAt: start}}}
	b := city.Build(snap, city.NewLayout()).Bridge()
	require.NotEmpty(t, b.Deck, "the bridge has no deck: nothing below can fail")
	span := b.Deck[0]
	under := span.Center()

	for _, heading := range []int{0, 90, 180, 270} {
		cam := city.NewCamera()
		cam.Heading = heading
		for _, deckFirst := range []bool{false, true} {
			got := namedOrder(cam, func(rec func(string) func()) []drawable {
				deck := bridgeItems(cam, b, func(cell city.Cell, abutment bool) func() {
					if cell == span {
						return rec("span")
					}
					return func() {}
				})
				moving := []drawable{
					carriageAt(cam, under, rec("train")),
					deliveryAt(cam, b, under, rec("truck")),
				}
				if deckFirst {
					return append(deck, moving...)
				}
				return append(moving, deck...)
			})
			at := func(name string) int { return slices.Index(got, name) }
			require.ElementsMatch(t, []string{"span", "train", "truck"}, got, "precondition: the first span, the train and the truck are each drawn once")

			t.Run(fmt.Sprintf("when a train stands under the first span, at heading %d, gathered deck first %v", heading, deckFirst), func(t *testing.T) {
				t.Run("it should draw the train before the deck", func(t *testing.T) {
					assert.Less(t, at("train"), at("span"), "%v", got)
				})

				t.Run("and a truck is on the deck above it", func(t *testing.T) {
					t.Run("it should draw the truck after the deck", func(t *testing.T) {
						assert.Greater(t, at("truck"), at("span"), "%v", got)
					})
				})
			})
		}
	}
}

// Item 132, after item 134 rescaled the train: the deck's clearance is
// the train's height, so nothing the loop carries reaches the deck's
// underside.
func TestBridgeClearance(t *testing.T) {
	k := testKits(t)
	g := &Game{kits: k}
	pieces := append(append([]string{}, kitLocos...), kitWagons...)

	t.Run("when the deck is raised over the rail", func(t *testing.T) {
		for _, atlas := range k.atlases {
			for _, heading := range []int{0, 90, 180, 270} {
				cam := city.NewCamera()
				cam.Zoom, cam.Heading = atlas.Zoom, heading
				require.Equal(t, atlas.Zoom, k.pick(cam.Zoom).Zoom, "precondition: the camera picks the atlas under test")
				t.Run(fmt.Sprintf("at zoom %.0f and heading %d, it should leave the deck's underside above every train piece's roof", atlas.Zoom, heading), func(t *testing.T) {
					var roof float64
					for _, piece := range pieces {
						for _, turn := range []int{0, 90, 180, 270} {
							roof = max(roof, g.kitRoofLiftTurned(cam, piece, turn))
						}
					}
					require.Positive(t, roof, "no train piece in the atlas: the check passes on nothing")
					assert.Greater(t, g.deckUnderside(cam), roof)
				})
			}
		}
	})
}
