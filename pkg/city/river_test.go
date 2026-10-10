package city_test

import (
	"botropolis.city/botropolis/pkg/plan"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

func TestRiver(t *testing.T) {
	t.Run("when a city with several districts is built", func(t *testing.T) {
		snap := snapshot(session("a", cinders, state.Working), session("b", botropolis, state.Working),
			session("c", "/p/c", state.Working), session("d", "/p/d", state.Working))
		c := city.Build(snap, city.NewLayout())
		require.NotEmpty(t, c.RiverCells)

		// The river is plan.RiverCols wide now, for the usage boats to
		// be read across.
		t.Run("it should run down the east edge of the map", func(t *testing.T) {
			west := c.Bounds().Max.X - city.CellSize*plan.RiverCols
			for _, r := range c.RiverCells {
				assert.GreaterOrEqual(t, r.Cell.Rect().Min.X, west-1e-9, "river at %v", r.Cell)
				assert.LessOrEqual(t, r.Cell.Rect().Max.X, c.Bounds().Max.X+1e-9, "river at %v", r.Cell)
			}
		})

		t.Run("it should run the full height", func(t *testing.T) {
			rows := map[int]bool{}
			for _, r := range c.RiverCells {
				rows[r.Cell.Row] = true
			}
			assert.Len(t, rows, int(c.Bounds().Height()/city.CellSize))
		})

		t.Run("it should keep every district off it", func(t *testing.T) {
			for _, r := range c.RiverCells {
				for _, d := range c.Districts {
					assert.False(t, d.Rect.Contains(r.Cell.Center()), "%s is on the river", d.Name)
				}
			}
		})

		t.Run("it should join every cell to a neighbour", func(t *testing.T) {
			for _, r := range c.RiverCells {
				assert.NotZero(t, r.Mask)
			}
		})

		t.Run("it should be the same river when built again", func(t *testing.T) {
			again := city.Build(snap, city.NewLayout())
			assert.Equal(t, c.RiverCells, again.RiverCells)
		})

		t.Run("it should answer for a cell on it", func(t *testing.T) {
			_, ok := c.River(c.RiverCells[0].Cell)
			assert.True(t, ok)
		})
	})

	t.Run("when the city is empty", func(t *testing.T) {
		t.Run("it should have no river", func(t *testing.T) {
			assert.Empty(t, city.Build(state.Snapshot{}, city.NewLayout()).RiverCells)
		})
	})
}
