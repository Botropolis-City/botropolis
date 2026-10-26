package render

import (
	"fmt"
	"math"
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
	dir := 1.0
	if b.Far.X < b.Land.X {
		dir = -1
	}
	midRamp := city.Point{X: b.Land.X + dir*b.RampRun()/2, Y: b.Land.Y}

	for _, heading := range []int{0, 90, 180, 270} {
		cam := city.NewCamera()
		cam.Heading = heading
		for _, deckFirst := range []bool{false, true} {
			got := namedOrder(cam, func(rec func(string) func()) []drawable {
				deck := bridgeItems(cam, b, func(p bridgePart) func() {
					switch {
					case p.Cell == span && p.span():
						return rec("span")
					case p.Rect.Contains(midRamp):
						return rec("ramp")
					}
					return func() {}
				})
				moving := []drawable{
					carriageAt(cam, under, rec("train")),
					deliveryAt(cam, b, under, rec("truck")),
					deliveryAt(cam, b, midRamp, rec("climbing")),
				}
				if deckFirst {
					return append(deck, moving...)
				}
				return append(moving, deck...)
			})
			at := func(name string) int { return slices.Index(got, name) }
			require.ElementsMatch(t, []string{"span", "ramp", "train", "truck", "climbing"}, got, "precondition: the first span, the ramp under the climbing truck, the train and both trucks are each drawn once")

			t.Run(fmt.Sprintf("when a train stands under the first span, at heading %d, gathered deck first %v", heading, deckFirst), func(t *testing.T) {
				t.Run("it should draw the train before the deck", func(t *testing.T) {
					assert.Less(t, at("train"), at("span"), "%v", got)
				})

				t.Run("and a truck is on the deck above it", func(t *testing.T) {
					t.Run("it should draw the truck after the deck", func(t *testing.T) {
						assert.Greater(t, at("truck"), at("span"), "%v", got)
					})
				})

				t.Run("and a truck is halfway up the ramp", func(t *testing.T) {
					t.Run("it should draw the truck after the ramp beneath it", func(t *testing.T) {
						assert.Greater(t, at("climbing"), at("ramp"), "%v", got)
					})
				})
			})
		}
	}

	// Item 150: a wagon on the rail a cell off the first span, on the far
	// side or the near side, against every part of the bridge it is
	// wholly behind or wholly in front of.
	parts := bridgeParts(b)
	partName := func(p bridgePart) string { return fmt.Sprintf("part %v from %.0f", p.Cell, p.Rect.Min.X) }
	for _, heading := range []int{0, 90, 180, 270} {
		cam := city.NewCamera()
		cam.Heading = heading
		north := city.Point{X: under.X, Y: under.Y - city.CellSize}
		south := city.Point{X: under.X, Y: under.Y + city.CellSize}
		behind, ahead := north, south
		if nearer(cam, north, south) {
			behind, ahead = south, north
		}
		for _, side := range []struct {
			name string
			at   city.Point
			// before is whether a part the wagon is wholly behind
			// (true) or wholly in front of (false) is the one checked.
			before bool
		}{{"behind", behind, true}, {"in front of", ahead, false}} {
			wagon := city.Rect{Min: city.Point{X: side.at.X - city.Tile/2, Y: side.at.Y - city.Tile/2}, Max: city.Point{X: side.at.X + city.Tile/2, Y: side.at.Y + city.Tile/2}}
			tw := cam.TurnRect(wagon)
			for _, deckFirst := range []bool{false, true} {
				got := namedOrder(cam, func(rec func(string) func()) []drawable {
					deck := bridgeItems(cam, b, func(p bridgePart) func() { return rec(partName(p)) })
					moving := []drawable{carriageAt(cam, side.at, rec("wagon"))}
					if deckFirst {
						return append(deck, moving...)
					}
					return append(moving, deck...)
				})
				at := func(name string) int { return slices.Index(got, name) }
				spanChecked := false
				for _, p := range parts {
					tp := cam.TurnRect(p.Rect)
					wagonBehind := city.BehindTurned(tw, tp) && !city.BehindTurned(tp, tw)
					wagonAhead := city.BehindTurned(tp, tw) && !city.BehindTurned(tw, tp)
					if (side.before && !wagonBehind) || (!side.before && !wagonAhead) {
						continue
					}
					spanChecked = spanChecked || (p.span() && p.Cell == span)
					p := p
					t.Run(fmt.Sprintf("when a wagon stands on the cell just %s the first span, at heading %d, gathered deck first %v, against the %s", side.name, heading, deckFirst, partName(p)), func(t *testing.T) {
						if side.before {
							t.Run("it should draw the wagon before it", func(t *testing.T) {
								assert.Less(t, at("wagon"), at(partName(p)), "%v", got)
							})
						} else {
							t.Run("it should draw the wagon after it", func(t *testing.T) {
								assert.Greater(t, at("wagon"), at(partName(p)), "%v", got)
							})
						}
					})
				}
				require.True(t, spanChecked, "precondition: the wagon %s the first span at heading %d is wholly on that side of it", side.name, heading)
			}
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
		for i, atlas := range k.atlases {
			for _, heading := range []int{0, 90, 180, 270} {
				cam := city.NewCamera()
				cam.Zoom, cam.Heading = zoomPicking(t, k, i), heading
				require.Equal(t, atlas.Zoom, k.pick(cam.Zoom).Zoom, "precondition: the camera picks the atlas under test")
				t.Run(fmt.Sprintf("at zoom %g and heading %d, it should leave the deck's underside above every train piece's roof", cam.Zoom, heading), func(t *testing.T) {
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

// Item 146, Aria 2026-10-19: a gentler ramp, longer and lighter, open
// underneath like a span rather than a solid wedge.
func TestBridgeRamp(t *testing.T) {
	k := testKits(t)
	g := &Game{kits: k}
	start := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	snap := state.Snapshot{At: start, Sessions: []state.Session{{ID: "a", CWD: "/p/a", State: state.Working, StartedAt: start}}}
	b := city.Build(snap, city.NewLayout()).Bridge()
	require.NotEmpty(t, b.Deck, "the bridge has no deck: nothing below can fail")
	var ramp []bridgePart
	for _, p := range bridgeParts(b) {
		if !p.span() {
			ramp = append(ramp, p)
		}
	}
	require.NotEmpty(t, ramp, "the bridge has no ramp: nothing below can fail")

	t.Run("when the road ramps up to the deck", func(t *testing.T) {
		for i := range k.atlases {
			for _, heading := range []int{0, 90, 180, 270} {
				cam := city.NewCamera()
				cam.Projection, cam.Zoom, cam.Heading = city.Isometric, zoomPicking(t, k, i), heading
				t.Run(fmt.Sprintf("at zoom %g and heading %d, it should climb no steeper than %.1f rise over run", cam.Zoom, heading, maxRampSlope), func(t *testing.T) {
					assert.LessOrEqual(t, liftToWorld(cam, g.deckLift(cam))/b.RampRun(), maxRampSlope)
				})
			}
		}

	})

	// Item 252: the second bridge's hump is cut and joined as the ramp is.
	large := largeCity(t)
	unit := func(h city.Height) float64 { return float64(h) }
	for _, c := range []struct {
		name   string
		bridge city.Bridge
	}{
		{"the one bridge of a small city", b},
		{"the first bridge of a large city", large.Bridges()[0]},
		{"the second bridge of a large city", large.Bridges()[1]},
	} {
		t.Run("when the road runs along "+c.name, func(t *testing.T) {
			parts := bridgeParts(c.bridge)
			require.NotEmpty(t, parts, "precondition: the bridge has parts")
			lift := func(along float64) float64 { return c.bridge.LiftAt(along, unit) }

			t.Run("it should cut every sloped piece into segments no longer than half a cell, each on its own pier", func(t *testing.T) {
				var long []bridgePart
				for _, p := range parts {
					if !p.span() && p.Rect.Max.X-p.Rect.Min.X > city.CellSize/2+1e-9 {
						long = append(long, p)
					}
				}
				assert.Empty(t, long)
			})
			t.Run("it should start the first part at the landing on the ground", func(t *testing.T) {
				assert.Equal(t, [2]float64{0, 0}, [2]float64{parts[0].From, lift(parts[0].From)})
			})
			t.Run("it should run each part on from where the last left off", func(t *testing.T) {
				var gaps []int
				for i := 1; i < len(parts); i++ {
					if math.Abs(parts[i].From-parts[i-1].To) > 1e-9 {
						gaps = append(gaps, i)
					}
				}
				assert.Empty(t, gaps)
			})
			t.Run("it should hold every span level from end to end", func(t *testing.T) {
				var tilted []bridgePart
				for _, p := range parts {
					if p.span() && lift(p.From) != lift(p.To) {
						tilted = append(tilted, p)
					}
				}
				assert.Empty(t, tilted)
			})
		})
	}
}

const maxRampSlope = 0.8
