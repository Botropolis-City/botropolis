package city_test

import (
	"math"
	"testing"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreets(t *testing.T) {
	t.Run("when two districts are joined by a road", func(t *testing.T) {
		snap := snapshot(session("a", cinders, state.Working), session("b", botropolis, state.Working))
		snap.Roads = []state.Road{{From: cinders, To: botropolis, Messages: 3, Sessions: 1}}
		c := city.Build(snap, city.NewLayout())
		require.Len(t, c.Streets, 1)
		street := c.Streets[0]

		t.Run("it should lay cells only outside the districts", func(t *testing.T) {
			require.NotEmpty(t, c.StreetCells)
			for _, sc := range c.StreetCells {
				for _, d := range c.Districts {
					assert.False(t, d.Rect.Contains(sc.Cell.Center()), "cell %v is inside %s", sc.Cell, d.Name)
				}
			}
		})

		t.Run("it should join every cell to at least one neighbour", func(t *testing.T) {
			for _, sc := range c.StreetCells {
				assert.NotZero(t, sc.Mask, "cell %v", sc.Cell)
			}
		})

		t.Run("it should run the path between the two districts", func(t *testing.T) {
			assert.Equal(t, &c.Roads[0], street.Road)
			assert.GreaterOrEqual(t, len(street.Path), 2)
		})

		t.Run("it should keep the street's cells on the grid", func(t *testing.T) {
			for _, p := range street.Path {
				assert.InDelta(t, 0, float64(int(p.X-city.BuildingSize/2)%int(city.BuildingSize)), 1e-9)
			}
		})
	})

	t.Run("when there are no roads", func(t *testing.T) {
		c := city.Build(snapshot(session("a", cinders, state.Working)), city.NewLayout())

		t.Run("it should carry no traffic", func(t *testing.T) {
			assert.Empty(t, c.Streets)
		})

		t.Run("it should still lay the avenue grid", func(t *testing.T) {
			assert.NotEmpty(t, c.Corridors)
		})
	})
}

func TestStreetsAroundDistricts(t *testing.T) {
	t.Run("when a third district sits between two that are joined", func(t *testing.T) {
		// Three districts fill the first grid row; the road joins the two
		// ends, and the middle one is in the way.
		snap := snapshot(session("a", "/p/a", state.Working), session("b", "/p/b", state.Working), session("c", "/p/c", state.Working))
		snap.Roads = []state.Road{{From: "/p/a", To: "/p/c", Messages: 2, Sessions: 1}}
		c := city.Build(snap, city.NewLayout())
		require.Len(t, c.Streets, 1)
		path := c.Streets[0].Path

		t.Run("it should stay contiguous, cell to cell, from driveway to driveway", func(t *testing.T) {
			for i := 2; i < len(path)-1; i++ {
				d := math.Hypot(path[i].X-path[i-1].X, path[i].Y-path[i-1].Y)
				assert.InDelta(t, city.BuildingSize, d, 1e-9, "gap between %v and %v", path[i-1], path[i])
			}
		})

		t.Run("it should not run through the district in the way", func(t *testing.T) {
			middle := c.Districts[1]
			for _, p := range path {
				assert.False(t, middle.Rect.Contains(p), "%v is inside %s", p, middle.Name)
			}
		})
	})
}

func TestStreetMasks(t *testing.T) {
	t.Run("when the avenue grid must turn a corner", func(t *testing.T) {
		// Four districts fill the first grid row and start a second; a road
		// from the third to the fourth has to bend.
		snap := snapshot(session("a", "/p/a", state.Working), session("b", "/p/b", state.Working),
			session("c", "/p/c", state.Working), session("d", "/p/d", state.Working))
		snap.Roads = []state.Road{{From: "/p/c", To: "/p/d", Messages: 1, Sessions: 1}}
		c := city.Build(snap, city.NewLayout())
		var corners int
		for _, sc := range c.Corridors {
			m := sc.Mask
			horizontal := m&(city.DirE|city.DirW) != 0
			vertical := m&(city.DirN|city.DirS) != 0
			if horizontal && vertical {
				corners++
			}
		}

		t.Run("it should have a cell that joins a horizontal and a vertical run", func(t *testing.T) {
			assert.GreaterOrEqual(t, corners, 1)
		})

		t.Run("it should bend clear of the other districts", func(t *testing.T) {
			for _, sc := range c.StreetCells {
				for _, d := range c.Districts {
					assert.False(t, d.Rect.Contains(sc.Cell.Center()), "cell %v is inside %s", sc.Cell, d.Name)
				}
			}
		})
	})
}
