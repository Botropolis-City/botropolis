package city

import "math"

// The fog over the sea where the bridge goes out (item 239): a bank over
// the haze, thickening toward the deck's far end so the deck and its
// trucks fade into something, and a thin low mist along that whole
// coast. It stands still and follows from the plan alone.

// FogPuff is one soft puff of the fog: where it stands, how high as a
// share of the deck's, how wide in cells, and how much of it shows.
type FogPuff struct {
	At     Point
	Lift   float64
	Radius float64
	Alpha  float64
	// Mist is a puff of the thin mist along the coast, not the bank.
	Mist bool
}

const (
	fogBankPuffs = 140
	// fogMistPerCell is how many mist puffs stand along each cell of
	// coast.
	fogMistPerCell = 5
)

// Fog is every puff of the first bridge's fog, back to front in no
// particular order.
func (c *City) Fog() []FogPuff {
	if _, ok := c.bridgeCell(); !ok {
		return nil
	}
	return c.FogOf(c.Bridge())
}

// FogOf is every puff of the fog a bridge's deck fades into: each bridge
// has its own (item 220).
func (c *City) FogOf(b Bridge) []FogPuff {
	if b.Land == b.Far {
		return nil
	}
	dir := math.Copysign(1, b.Far.X-b.Land.X)
	band := math.Abs(b.Far.X - b.Haze.X)
	lead, tail := 0.4*CellSize, 1.2*CellSize
	length := lead + band + tail
	puffs := make([]FogPuff, 0, fogBankPuffs)
	for i := range fogBankPuffs {
		k := uint64(i)
		s := fogHash(k, 1)
		into := s*length - lead
		u := into / band
		spread := (0.5 + 0.9*smoothstep(u)) * CellSize
		puffs = append(puffs, FogPuff{
			At:     Point{X: b.Haze.X + dir*into, Y: b.Land.Y + (fogHash(k, 2)*2-1)*spread},
			Lift:   0.35 + 1.1*fogHash(k, 3),
			Radius: 0.5 + 0.9*fogHash(k, 4),
			Alpha:  0.18 + 0.4*smoothstep(u),
		})
	}
	reach := c.Island().Reach()
	lo, hi := reach.Min.Y-2*CellSize, reach.Max.Y+2*CellSize
	n := int((hi-lo)/CellSize) * fogMistPerCell
	for i := range n {
		k := uint64(i + fogBankPuffs)
		y := lo + (float64(i)+fogHash(k, 2))/float64(n)*(hi-lo)
		edge := min(y-lo, hi-y) / (2 * CellSize)
		puffs = append(puffs, FogPuff{
			At:     Point{X: b.Haze.X + dir*(fogHash(k, 1)*2.5-0.5)*CellSize, Y: y},
			Lift:   0.1 + 0.5*fogHash(k, 3),
			Radius: 1.4 + 1.2*fogHash(k, 4),
			Alpha:  (0.1 + 0.1*fogHash(k, 5)) * smoothstep(edge),
			Mist:   true,
		})
	}
	return puffs
}

// fogHash is a number in [0, 1) for a puff and a property of it.
func fogHash(i, salt uint64) float64 {
	x := i*0x9e3779b97f4a7c15 + salt*0xbf58476d1ce4e5b9
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	x ^= x >> 31
	return float64(x>>11) / float64(1<<53)
}

func smoothstep(t float64) float64 {
	t = min(1, max(0, t))
	return t * t * (3 - 2*t)
}
