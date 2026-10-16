package city_test

import (
	"testing"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/plan"
	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 85, slice 4: the trees that mean nothing are ground. The belt's
// wood on the corridors gives way where a road is laid, and nothing on
// the ground answers a hover.

func TestGroundTreesFollowRoads(t *testing.T) {
	t.Run("when a road is laid on a corridor", func(t *testing.T) {
		snap := snapshot(threeDistricts()...)
		snap.Roads = []state.Road{{From: "/p/a", To: "/p/c", Files: 2, Sessions: 1}}
		c := city.Build(snap, city.NewLayout())
		road := laid(c)
		require.NotEmpty(t, road)

		t.Run("it should stand no ground tree on a laid road cell", func(t *testing.T) {
			for _, tree := range c.Trees {
				if tree.Kind != plan.ParkTree {
					continue
				}
				_, onRoad := road[city.CellOf(tree.At)]
				assert.False(t, onRoad, "tree at %v", tree.At)
			}
		})
	})
}

func TestTreeHover(t *testing.T) {
	s := city.NewScene(city.NewLayout())
	s.Resize(1600, 1200)
	s.SetSnapshot(snapshot(threeDistricts()...))
	var tree *city.Tree
	for i := range s.City().Trees {
		if s.City().Trees[i].Kind == plan.ParkTree {
			tree = &s.City().Trees[i]
			break
		}
	}
	require.NotNil(t, tree)

	t.Run("when the pointer is on a ground tree", func(t *testing.T) {
		s.PointerMove(s.Camera().WorldToScreen(tree.At))

		t.Run("it should select nothing", func(t *testing.T) {
			assert.Equal(t, city.Hit{}, s.Hover())
		})

		t.Run("it should show no card", func(t *testing.T) {
			_, shown := s.Card()
			assert.False(t, shown)
		})
	})
}
