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

// Item 252, Aria 2026-10-25: "Raise the deck". The second bridge's deck
// rises from each bank of the river to clear the tallest boat, and comes
// back down to its old height before the haze; a bridge over no river
// keeps item 146's one ramp.
func TestBridgeProfile(t *testing.T) {
	levels := func(h city.Height) float64 {
		return map[city.Height]float64{city.AtGround: 0, city.AtDeck: 1, city.AtChannel: 2}[h]
	}
	cityOf := func(n int) *city.City {
		var sessions []state.Session
		for i := 0; i < n; i++ {
			sessions = append(sessions, session(fmt.Sprintf("s%02d", i), fmt.Sprintf("/p/d%02d", i), state.Working))
		}
		return build(t, city.NewLayout(), sessions...)
	}
	small, large := cityOf(1), cityOf(city.SecondBridgeAt+1)
	require.Len(t, small.Bridges(), 1, "precondition: a small city has one bridge")
	require.Len(t, large.Bridges(), 2, "precondition: a large city has two")
	out := func(b city.Bridge, along float64) city.Point {
		return city.Point{X: b.Land.X + math.Copysign(along, b.Far.X-b.Land.X), Y: b.Land.Y}
	}
	alongOf := func(b city.Bridge, x float64) float64 { return math.Abs(x - b.Land.X) }

	for _, c := range []struct {
		name   string
		bridge city.Bridge
	}{
		{"the one bridge of a small city", small.Bridge()},
		{"the first bridge of a large city", large.Bridges()[0]},
	} {
		t.Run("when "+c.name+" crosses no river", func(t *testing.T) {
			b := c.bridge
			require.NotEmpty(t, b.Deck, "precondition: the bridge has a deck")

			t.Run("it should have no channel", func(t *testing.T) {
				assert.Equal(t, city.Span{}, b.Channel)
			})
			t.Run("it should climb from the ground at the landing to deck level where the first span starts, and stay there to the far end", func(t *testing.T) {
				assert.Equal(t, []city.Knot{
					{Along: 0, Height: city.AtGround},
					{Along: b.RampRun(), Height: city.AtDeck},
					{Along: alongOf(b, b.Far.X), Height: city.AtDeck},
				}, b.Profile())
			})
			// Item 146's cases, kept.
			for _, at := range []struct {
				name string
				p    city.Point
				want float64
			}{
				{"it should be at ground level at the landing's centre", b.Land, 0},
				{"it should be halfway up halfway along the ramp", out(b, b.RampRun()/2), 0.5},
				{"it should be at deck level where the first span starts", out(b, b.RampRun()), 1},
				{"it should be at deck level at the far end", b.Far, 1},
				{"it should be at ground level off the bridge's row", city.Point{X: out(b, b.RampRun()).X, Y: b.Land.Y + city.CellSize}, 0},
			} {
				t.Run(at.name, func(t *testing.T) {
					assert.InDelta(t, at.want, b.Lift(at.p, levels), 1e-9)
				})
			}
		})
	}

	t.Run("when the second bridge crosses the river", func(t *testing.T) {
		b := large.Bridges()[1]
		west, east := riverBandEdges(t, large)
		near, far := alongOf(b, west), alongOf(b, east)
		if near > far {
			near, far = far, near
		}
		climb := near - b.RampRun()
		profile := b.Profile()
		at := func(along float64) float64 { return b.Lift(out(b, along), levels) }

		t.Run("it should hold the channel from the river band's west edge to its east edge", func(t *testing.T) {
			assert.Equal(t, city.Span{From: west, To: east}, b.Channel)
		})
		t.Run("it should reach deck level where the first span starts, over the rail", func(t *testing.T) {
			assert.InDelta(t, 1, at(b.RampRun()), 1e-9)
		})
		t.Run("it should climb from deck level to the channel over the cells before the river", func(t *testing.T) {
			assert.Positive(t, climb)
		})
		for _, f := range []float64{0, 0.5, 1} {
			t.Run(fmt.Sprintf("it should be at channel level %.0f%% of the way across the channel", 100*f), func(t *testing.T) {
				assert.InDelta(t, 2, at(near+f*(far-near)), 1e-9)
			})
		}
		t.Run("it should come back down to deck level as far past the channel as it climbed before it", func(t *testing.T) {
			assert.Equal(t, [2]float64{1.5, 1}, [2]float64{at(far + climb/2), at(far + climb)})
		})
		t.Run("it should be back at deck level before the haze", func(t *testing.T) {
			assert.LessOrEqual(t, far+climb, alongOf(b, b.Haze.X))
		})
		t.Run("it should stay at deck level to the far end", func(t *testing.T) {
			assert.Equal(t, city.Knot{Along: alongOf(b, b.Far.X), Height: city.AtDeck}, profile[len(profile)-1])
		})
		t.Run("and a truck drives it, it should ride at channel level over every river cell", func(t *testing.T) {
			var over, low []city.Cell
			for _, cell := range b.Deck {
				if _, river := large.River(cell); river {
					over = append(over, cell)
					if b.Lift(cell.Center(), levels) != 2 {
						low = append(low, cell)
					}
				}
			}
			require.NotEmpty(t, over, "precondition: the deck stands over the river")
			assert.Empty(t, low)
		})
	})
}

// riverBandEdges is the river band's west and east edges, where the
// boats sail.
func riverBandEdges(t *testing.T, c *city.City) (west, east float64) {
	t.Helper()
	from, _, width, ok := c.RiverBand()
	require.True(t, ok, "precondition: the city has a river")
	return from.X - width/2, from.X + width/2
}
