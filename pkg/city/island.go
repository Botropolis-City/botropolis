package city

import (
	"math"

	"botropolis.city/botropolis/pkg/plan"
)

// The plan stands on an island. The coast lies a little way past the
// plan's edge, wandering by a smooth noise so it reads as a shore rather
// than a cut, with a beach on the land side and shallows on the sea's;
// far out, a faint far shore. The river runs through the island and out
// to the sea at both ends. It all follows from the plan's bounds and the
// river's columns, so the coast grows with the city and the same plan
// always has the same coast. Item 85.

type Ground int

const (
	GroundLand Ground = iota
	GroundSand
	GroundShallows
	GroundSea
	GroundFarShore
	GroundRiver
)

func (g Ground) String() string {
	return [...]string{"land", "sand", "shallows", "sea", "far shore", "river"}[g]
}

// In cells: how far past the plan the coast lies on average, how far it
// wanders either way, how wide the beach and the shallows are, and where
// the far shore stands and how deep its band is.
const (
	coastCells    = 2.0
	coastWobble   = 1.1
	sandCells     = 0.35
	shallowsCells = 1.6
	farShoreAt    = 24.0
	farShoreBand  = 5.0
	// SeaCells is how far round the plan the sea is drawn.
	SeaCells = 34.0
)

type Island struct {
	plan           Rect
	riverW, riverE float64
	hasRiver       bool
}

func (c *City) Island() Island {
	i := Island{plan: c.bounds}
	if from, _, width, ok := c.RiverBand(); ok {
		i.riverW, i.riverE, i.hasRiver = from.X-width/2, from.X+width/2, true
	}
	return i
}

// Plan is the plan the island stands round.
func (i Island) Plan() Rect { return i.plan }

// Reach is the furthest the coast and its shallows can lie from the plan.
func (i Island) Reach() Rect {
	return grow(i.plan, (coastCells+coastWobble+shallowsCells)*CellSize)
}

// Sea is how far round the plan the sea is drawn.
func (i Island) Sea() Rect { return grow(i.plan, SeaCells*CellSize) }

// Mouths are where the river meets the sea, just past the coast's
// furthest reach: tugs come in at the north one and leave by the south.
func (i Island) Mouths() (north, south Point) {
	r := i.Reach()
	x := (i.riverW + i.riverE) / 2
	return Point{X: x, Y: r.Min.Y}, Point{X: x, Y: r.Max.Y}
}

// At is the ground at a world point, and how far across its band the
// point lies, from 0 at the band's landward edge to 1 at its seaward.
func (i Island) At(p Point) (Ground, float64) {
	d := outside(i.plan, p)
	inRiver := i.hasRiver && p.X >= i.riverW && p.X < i.riverE
	if d == 0 {
		if inRiver {
			return GroundRiver, 0
		}
		return GroundLand, 0
	}
	e := d/CellSize - (coastCells + coastWobble*coastNoise(p))
	if inRiver && e < 0 {
		return GroundRiver, 0
	}
	switch {
	case e < -sandCells:
		return GroundLand, 0
	case e < 0:
		return GroundSand, 1 + e/sandCells
	case e < shallowsCells:
		return GroundShallows, e / shallowsCells
	}
	fd := d/CellSize - farShoreAt - 2*valueNoise(p.X/(6*CellSize), p.Y/(6*CellSize))
	if fd > 0 && fd < farShoreBand {
		return GroundFarShore, math.Sin(math.Pi * fd / farShoreBand)
	}
	return GroundSea, 0
}

func grow(r Rect, by float64) Rect {
	return Rect{Min: Point{X: r.Min.X - by, Y: r.Min.Y - by}, Max: Point{X: r.Max.X + by, Y: r.Max.Y + by}}
}

func outside(r Rect, p Point) float64 {
	dx := math.Max(0, math.Max(r.Min.X-p.X, p.X-r.Max.X))
	dy := math.Max(0, math.Max(r.Min.Y-p.Y, p.Y-r.Max.Y))
	return math.Hypot(dx, dy)
}

var valueNoise = plan.Noise

func coastNoise(p Point) float64 {
	return 0.65*valueNoise(p.X/(2.5*CellSize), p.Y/(2.5*CellSize)) + 0.35*valueNoise(p.X/(0.9*CellSize)+17, p.Y/(0.9*CellSize)+5)
}
