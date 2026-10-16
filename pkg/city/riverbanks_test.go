package city_test

import (
	"fmt"
	"math"
	"testing"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 85. Aria: the river was "too straight when the island is wavy
// edges, and it doesn't blend into the ocean on either side because the
// shade of blue is just cut square at the ends." The band the gauges hold
// station in is unchanged; only the drawn water widens past it.

// bankWander is the least a bank's lateral offset must vary along the
// plan, in cells, for it not to read as a ruled line.
const bankWander = 0.15

func isRiver(i city.Island, p city.Point) bool {
	g, _ := i.At(p)
	return g == city.GroundRiver
}

// banks walks outward from the band at height y while the water lasts,
// and returns how far it reached on each side and every point it crossed.
func banks(i city.Island, west, east, y float64) (float64, float64, []city.Point) {
	const step = 1.0
	var wet []city.Point
	w := west
	for isRiver(i, city.Point{X: w - step, Y: y}) && w-west > -2*city.CellSize {
		w -= step
		wet = append(wet, city.Point{X: w, Y: y})
	}
	e := east
	for isRiver(i, city.Point{X: e + step, Y: y}) && e-east < 2*city.CellSize {
		e += step
		wet = append(wet, city.Point{X: e, Y: y})
	}
	return w, e, wet
}

func TestRiverBanks(t *testing.T) {
	c := city.Build(snapshot(districts(9)...), city.NewLayout())
	island := c.Island()
	from, to, width, ok := c.RiverBand()
	require.True(t, ok)
	west, east := from.X-width/2, from.X+width/2

	t.Run("when the west bank is followed the length of the plan", func(t *testing.T) {
		lo, hi := math.Inf(1), math.Inf(-1)
		for y := from.Y; y <= to.Y; y += city.CellSize / 4 {
			w, _, _ := banks(island, west, east, y)
			lo, hi = math.Min(lo, w), math.Max(hi, w)
		}

		t.Run("it should wander, not run as one straight line", func(t *testing.T) {
			assert.GreaterOrEqual(t, (hi-lo)/city.CellSize, bankWander)
		})
	})

	t.Run("when the same plan is asked twice", func(t *testing.T) {
		again := city.Build(snapshot(districts(9)...), city.NewLayout()).Island()

		t.Run("it should draw the same banks", func(t *testing.T) {
			for y := from.Y; y <= to.Y; y += city.CellSize / 4 {
				a, b, _ := banks(island, west, east, y)
				c, d, _ := banks(again, west, east, y)
				assert.Equal(t, [2]float64{a, b}, [2]float64{c, d}, "at y %.0f", y)
			}
		})
	})

	t.Run("in every row", func(t *testing.T) {
		t.Run("it should be water across the whole gauge band", func(t *testing.T) {
			for y := from.Y; y <= to.Y; y += city.CellSize / 4 {
				for x := west + 0.5; x < east; x += 2 {
					assert.True(t, isRiver(island, city.Point{X: x, Y: y}), "dry at (%.0f, %.0f)", x, y)
				}
			}
		})
	})

	b := c.Bounds()
	for _, mouth := range []struct {
		name        string
		edge, coast float64
	}{
		{"north", b.Min.Y - 1, b.Min.Y - 1.5*city.CellSize},
		{"south", b.Max.Y + 1, b.Max.Y + 1.5*city.CellSize},
	} {
		t.Run("at the "+mouth.name+" mouth", func(t *testing.T) {
			w0, e0, _ := banks(island, west, east, mouth.edge)
			w1, e1, _ := banks(island, west, east, mouth.coast)

			t.Run("it should widen toward the sea", func(t *testing.T) {
				assert.Greater(t, e1-w1, e0-w0+city.CellSize/2)
			})
		})
	}
}

func TestRiverKeepsClear(t *testing.T) {
	for _, n := range []int{1, 9, 25, 49} {
		c := city.Build(snapshot(districts(n)...), city.NewLayout())
		island := c.Island()
		from, to, width, _ := c.RiverBand()
		west, east := from.X-width/2, from.X+width/2
		rails := map[city.Cell]bool{}
		for _, r := range c.Plan().Rails {
			rails[city.Cell{Col: r.Col, Row: r.Row}] = true
		}
		corridors := map[city.Cell]bool{}
		for _, sc := range c.Corridors {
			corridors[sc.Cell] = true
		}

		t.Run(fmt.Sprintf("when the city has %d rings", c.Rings()), func(t *testing.T) {
			t.Run("it should reach no block, road or rail", func(t *testing.T) {
				for y := from.Y; y <= to.Y; y += city.CellSize / 4 {
					_, _, wet := banks(island, west, east, y)
					for _, p := range wet {
						cell := city.CellOf(p)
						assert.False(t, rails[cell] || corridors[cell], "water on rail or road at %v", p)
						for _, d := range c.Districts {
							assert.False(t, d.Rect.Contains(p), "water in %s at %v", d.Name, p)
						}
					}
				}
			})
		})
	}
}

func TestRiverAndTrees(t *testing.T) {
	c := city.Build(snapshot(districts(9)...), city.NewLayout())
	island := c.Island()

	t.Run("it should stand no ground tree in the river's water", func(t *testing.T) {
		for _, tree := range c.Trees {
			if tree.Kind == plan.ParkTree {
				assert.False(t, isRiver(island, tree.At), "tree at %v", tree.At)
			}
		}
	})
}
