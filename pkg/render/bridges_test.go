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

// Item 220: once the island holds 16 districts there are two bridges,
// and each is drawn, with a fog bank of its own out past its coast.
func TestEveryBridgeIsDrawn(t *testing.T) {
	start := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	var sessions []state.Session
	for i := 0; i < city.SecondBridgeAt; i++ {
		sessions = append(sessions, state.Session{ID: fmt.Sprintf("s%02d", i), CWD: fmt.Sprintf("/p/d%02d", i), State: state.Working, Alive: true, StartedAt: start})
	}
	c := city.Build(state.Snapshot{At: start, Sessions: sessions}, city.NewLayout())
	bridges := c.Bridges()
	require.Len(t, bridges, 2, "precondition: the city has two bridges")
	cam := city.NewCamera()
	cam.Projection, cam.Zoom = city.Isometric, 1

	parts := map[int]int{}
	var banks [][]city.FogPuff
	items := bridgesInSort(cam, c, func(i int, _ city.Bridge, _ bridgePart) func() {
		parts[i]++
		return func() {}
	}, func(puffs []city.FogPuff) func() {
		banks = append(banks, puffs)
		return func() {}
	}, nil, nil)
	require.NotEmpty(t, items)

	// Aria, 2026-10-25: the second bridge crosses the river on its raised
	// deck, and tugs and ships pass under it.
	t.Run("when a boat sails the river under the second bridge", func(t *testing.T) {
		b := bridges[1]
		island := c.Island()
		var over []city.Cell
		for _, cell := range b.Deck {
			if g, _ := island.At(cell.Center()); g == city.GroundRiver {
				over = append(over, cell)
			}
		}
		require.NotEmpty(t, over, "precondition: the deck stands over the river")
		from, to, _, ok := c.GaugeRun(gaugeEndroom)
		require.True(t, ok, "precondition: the river has a run for the boats")

		t.Run("the boats' run should pass under the deck", func(t *testing.T) {
			lo, hi := math.Min(from.Y, to.Y), math.Max(from.Y, to.Y)
			assert.True(t, b.Land.Y > lo && b.Land.Y < hi, "the deck at y=%v, the run from %v to %v", b.Land.Y, lo, hi)
		})

		cell := over[len(over)/2]
		for _, heading := range []int{0, 90, 180, 270} {
			cam := city.NewCamera()
			cam.Heading = heading
			for _, deckFirst := range []bool{false, true} {
				got := namedOrder(cam, func(rec func(string) func()) []drawable {
					deck := bridgeItems(cam, b, func(p bridgePart) func() {
						if p.Cell == cell && p.span() {
							return rec("span")
						}
						return func() {}
					})
					boat := standingAt(cam, cell.Center(), rec("boat"))
					boat.moves = true
					if deckFirst {
						return append(deck, boat)
					}
					return append([]drawable{boat}, deck...)
				})
				require.ElementsMatch(t, []string{"span", "boat"}, got, "precondition: the span over the river and the boat are each drawn once")

				t.Run(fmt.Sprintf("at heading %d, gathered deck first %v, it should draw the boat before the deck", heading, deckFirst), func(t *testing.T) {
					assert.Less(t, slices.Index(got, "boat"), slices.Index(got, "span"), "%v", got)
				})
			}
		}
	})

	// Item 252: over the channel the deck is raised, and a truck on it
	// rides up there and draws over it.
	t.Run("when a truck drives over the channel on the second bridge", func(t *testing.T) {
		b := bridges[1]
		require.NotEqual(t, city.Span{}, b.Channel, "precondition: the second bridge crosses the river")
		at := city.Point{X: (b.Channel.From + b.Channel.To) / 2, Y: b.Land.Y}
		k := preparedKits(t)
		g := &Game{kits: k}
		cam := city.NewCamera()
		cam.Projection, cam.Zoom = city.Isometric, 1

		t.Run("it should ride at the channel's height", func(t *testing.T) {
			assert.InDelta(t, g.heights(cam)(city.AtChannel), g.roadLift(cam, b, at), 1e-9)
		})
		for _, heading := range []int{0, 90, 180, 270} {
			cam := city.NewCamera()
			cam.Heading = heading
			got := namedOrder(cam, func(rec func(string) func()) []drawable {
				deck := bridgeItems(cam, b, func(p bridgePart) func() {
					if p.span() && p.Cell == city.CellOf(at) {
						return rec("span")
					}
					return func() {}
				})
				truck := deliveryAt(cam, b, at, rec("truck"))
				truck.moves = true
				return append([]drawable{truck}, deck...)
			})
			require.ElementsMatch(t, []string{"span", "truck"}, got, "precondition: the span under the truck and the truck are each drawn once")

			t.Run(fmt.Sprintf("at heading %d, it should draw the truck after the span beneath it", heading), func(t *testing.T) {
				assert.Greater(t, slices.Index(got, "truck"), slices.Index(got, "span"), "%v", got)
			})
		}
	})

	for i, b := range bridges {
		t.Run(fmt.Sprintf("for bridge %d", i+1), func(t *testing.T) {
			t.Run("it should put its parts in the depth sort", func(t *testing.T) {
				assert.Equal(t, len(bridgeParts(b)), parts[i])
			})

			t.Run("it should have a fog bank of its own, out past its coast", func(t *testing.T) {
				own := 0
				for _, bank := range banks {
					mean := 0.0
					for _, p := range bank {
						mean += p.At.X
					}
					mean /= float64(len(bank))
					if math.Signbit(mean-b.Land.X) == math.Signbit(b.Far.X-b.Land.X) && math.Abs(mean-b.Land.X) > math.Abs(b.Haze.X-b.Land.X)-2*city.CellSize {
						own++
					}
				}
				assert.Equal(t, 1, own)
			})
		})
	}
}
