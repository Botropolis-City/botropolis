package city_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 132, Aria 2026-10-19: the rail stays on the ground and the road
// goes over it. The landing cell is the bridge's abutment, the first
// span is over the rail, and the deck runs out to sea from there.
func TestBridgeOverTheRail(t *testing.T) {
	c := build(t, city.NewLayout(), session("a", cinders, state.Working), session("b", mullet, state.Resting))
	bridge := c.Bridge()
	rail := map[city.Cell]bool{}
	for _, p := range c.Rails {
		rail[city.CellOf(p)] = true
	}
	require.NotEmpty(t, rail, "precondition: the city has a freight loop")
	require.NotEmpty(t, bridge.Deck, "the bridge has no deck: every check below passes on nothing")
	far := city.CellOf(bridge.Far)
	step := 1
	if far.Col < bridge.Abutment.Col {
		step = -1
	}

	t.Run("when the city has a bridge and a freight loop", func(t *testing.T) {
		t.Run("it should start the bridge at an abutment on the landing cell", func(t *testing.T) {
			assert.Equal(t, city.CellOf(bridge.Land), bridge.Abutment)
		})

		t.Run("and the landing is an avenue cell", func(t *testing.T) {
			require.True(t, c.IsStreet(bridge.Abutment), "precondition: the landing is on an avenue")

			t.Run("it should put the rail between the abutment and the far end", func(t *testing.T) {
				between := false
				for col := bridge.Abutment.Col + step; col != far.Col; col += step {
					between = between || rail[city.Cell{Col: col, Row: bridge.Abutment.Row}]
				}
				assert.True(t, between)
			})
		})

		t.Run("it should make the first span the cell over the rail", func(t *testing.T) {
			assert.True(t, rail[bridge.Deck[0]], "first span %v", bridge.Deck[0])
		})

		// Item 146: the road ramps up over the cells between the abutment
		// and the rail, then stays on the deck out to the far end.
		t.Run("it should make every cell from the abutment out to the far end ramp, then deck", func(t *testing.T) {
			var want []city.Cell
			for col := bridge.Abutment.Col + step; col != far.Col+step; col += step {
				want = append(want, city.Cell{Col: col, Row: bridge.Abutment.Row})
			}
			assert.Equal(t, want, append(append([]city.Cell{}, bridge.Ramp...), bridge.Deck...))
		})

		t.Run("it should ramp over at least one whole cell before the rail", func(t *testing.T) {
			assert.NotEmpty(t, bridge.Ramp)
		})

		t.Run("it should lay no street cell on any ramp or deck cell", func(t *testing.T) {
			deck := map[city.Cell]bool{}
			for _, cell := range append(append([]city.Cell{}, bridge.Ramp...), bridge.Deck...) {
				deck[cell] = true
			}
			var on []city.Cell
			for _, sc := range c.StreetCells {
				if deck[sc.Cell] {
					on = append(on, sc.Cell)
				}
			}
			assert.Empty(t, on)
		})

		for _, n := range []int{1, 6, 20} {
			t.Run(fmt.Sprintf("and the city has %d sessions, it should plant no tree on the abutment, the ramp or under the deck", n), func(t *testing.T) {
				var sessions []state.Session
				for i := 0; i < n; i++ {
					sessions = append(sessions, session(fmt.Sprint("s", i), fmt.Sprint("/p/", i), state.Working))
				}
				c := build(t, city.NewLayout(), sessions...)
				b := c.Bridge()
				under := map[city.Cell]bool{b.Abutment: true}
				for _, cell := range append(append([]city.Cell{}, b.Ramp...), b.Deck...) {
					under[cell] = true
				}
				var planted []city.Cell
				for _, tree := range c.Trees {
					if under[city.CellOf(tree.At)] {
						planted = append(planted, city.CellOf(tree.At))
					}
				}
				assert.Empty(t, planted)
			})
		}

		t.Run("and a truck drives the bridge", func(t *testing.T) {
			top := bridge.Deck[0].Rect().Max.X
			if step > 0 {
				top = bridge.Deck[0].Rect().Min.X
			}
			spanStart := city.Point{X: top, Y: bridge.Land.Y}
			t.Run("it should run the ramp from the landing's centre to the first span", func(t *testing.T) {
				assert.InDelta(t, math.Abs(spanStart.X-bridge.Land.X), bridge.RampRun(), 1e-9)
			})
			for _, at := range []struct {
				name string
				p    city.Point
				want float64
			}{
				{"it should be at ground level at the landing's centre", bridge.Land, 0},
				{"it should be halfway up halfway along the ramp", city.Point{X: (bridge.Land.X + spanStart.X) / 2, Y: bridge.Land.Y}, 0.5},
				{"it should reach deck level where the first span starts", spanStart, 1},
				{"it should be at deck level over the rail", bridge.Deck[0].Center(), 1},
				{"it should be at deck level at the far end", bridge.Far, 1},
			} {
				t.Run(at.name, func(t *testing.T) {
					assert.InDelta(t, at.want, bridge.Lift(at.p, func(h city.Height) float64 { return float64(h) }), 1e-9)
				})
			}
		})
	})
}
