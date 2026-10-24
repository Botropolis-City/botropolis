package city_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 239, Aria 2026-10-24: the deck fades into something. A bank of fog
// stands over the haze where the bridge goes out, and a thin mist runs
// along that whole coast, still, over open sea.

func TestFogOverTheHaze(t *testing.T) {
	c := build(t, city.NewLayout(), session("a", cinders, state.Working), session("b", mullet, state.Resting))
	bridge := c.Bridge()
	require.NotEmpty(t, bridge.Deck, "the bridge has no deck: every check below passes on nothing")
	fog := c.Fog()
	var bank, mist []city.FogPuff
	for _, p := range fog {
		if p.Mist {
			mist = append(mist, p)
		} else {
			bank = append(bank, p)
		}
	}
	require.NotEmpty(t, bank, "no bank: nothing below can fail")
	require.NotEmpty(t, mist, "no mist: nothing below can fail")
	out := math.Copysign(1, bridge.Far.X-bridge.Land.X)
	along := func(p city.Point) float64 { return (p.X - bridge.Land.X) * out }
	on := func(d float64) city.Point { return city.Point{X: bridge.Land.X + out*d, Y: bridge.Land.Y} }
	under := func(puffs []city.FogPuff, p city.Point) float64 {
		sum := 0.0
		for _, f := range puffs {
			if math.Hypot(p.X-f.At.X, p.Y-f.At.Y) <= f.Radius*city.CellSize {
				sum += f.Alpha
			}
		}
		return sum
	}

	t.Run("when the city has a bridge", func(t *testing.T) {
		t.Run("it should cover the deck's fading span with the bank", func(t *testing.T) {
			var bare []float64
			for d := along(bridge.Haze); d <= along(bridge.Far)+1e-9; d += city.CellSize / 4 {
				if under(bank, on(d)) == 0 {
					bare = append(bare, d)
				}
			}
			assert.Empty(t, bare)
		})

		t.Run("it should thicken toward the far end", func(t *testing.T) {
			mean := func(p city.Point) float64 {
				sum, n := 0.0, 0
				for _, f := range bank {
					if math.Hypot(p.X-f.At.X, p.Y-f.At.Y) <= f.Radius*city.CellSize {
						sum, n = sum+f.Alpha, n+1
					}
				}
				return sum / float64(max(n, 1))
			}
			assert.Greater(t, mean(bridge.Far), mean(bridge.Haze))
		})

		t.Run("it should end the bank within a cell and a half past the far end", func(t *testing.T) {
			var past []city.Point
			for _, p := range bank {
				if along(p.At) > along(bridge.Far)+1.5*city.CellSize {
					past = append(past, p.At)
				}
			}
			assert.Empty(t, past)
		})

		t.Run("it should lay the mist along the bridge's whole coast", func(t *testing.T) {
			reach := c.Island().Reach()
			lo, hi := mist[0].At.Y, mist[0].At.Y
			for _, p := range mist {
				lo, hi = min(lo, p.At.Y), max(hi, p.At.Y)
			}
			assert.Equal(t, [2]bool{true, true}, [2]bool{lo <= reach.Min.Y, hi >= reach.Max.Y})
		})

		t.Run("it should keep every puff over open sea", func(t *testing.T) {
			var ashore []city.Point
			for _, p := range fog {
				if g, _ := c.Island().At(p.At); g != city.GroundSea {
					ashore = append(ashore, p.At)
				}
			}
			assert.Empty(t, ashore)
		})

		t.Run("it should place the same fog for the same plan", func(t *testing.T) {
			again := build(t, city.NewLayout(), session("a", cinders, state.Working), session("b", mullet, state.Resting))
			assert.Equal(t, fog, again.Fog())
		})
	})

	t.Run("when the city has no bridge", func(t *testing.T) {
		t.Run("it should have no fog", func(t *testing.T) {
			assert.Empty(t, (&city.City{}).Fog())
		})
	})
}
