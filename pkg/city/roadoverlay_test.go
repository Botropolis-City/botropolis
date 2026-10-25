package city_test

import (
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/plan"
	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 85. The avenue grid stays in the plan as corridors, and a road
// is laid on a corridor only where a connection runs.

func laid(c *city.City) map[city.Cell]int {
	cells := map[city.Cell]int{}
	for _, sc := range c.StreetCells {
		cells[sc.Cell] = sc.Mask
	}
	return cells
}

func corridors(c *city.City) map[city.Cell]bool {
	cells := map[city.Cell]bool{}
	for _, sc := range c.Corridors {
		cells[sc.Cell] = true
	}
	return cells
}

func neighbours(cell city.Cell) map[int]city.Cell {
	return map[int]city.Cell{
		city.DirN: {Col: cell.Col, Row: cell.Row - 1},
		city.DirE: {Col: cell.Col + 1, Row: cell.Row},
		city.DirS: {Col: cell.Col, Row: cell.Row + 1},
		city.DirW: {Col: cell.Col - 1, Row: cell.Row},
	}
}

func streetTrees(c *city.City) []city.Tree {
	var out []city.Tree
	for _, tree := range c.Trees {
		if tree.Kind == plan.StreetTree {
			out = append(out, tree)
		}
	}
	return out
}

func threeDistricts() []state.Session {
	return []state.Session{session("a", "/p/a", state.Working), session("b", "/p/b", state.Working), session("c", "/p/c", state.Working)}
}

func TestRoadOverlay(t *testing.T) {
	t.Run("when there are no connections", func(t *testing.T) {
		c := city.Build(snapshot(threeDistricts()...), city.NewLayout())

		t.Run("it should keep the whole avenue grid as corridors", func(t *testing.T) {
			assert.NotEmpty(t, c.Corridors)
		})

		t.Run("it should lay no road but the bridge's", func(t *testing.T) {
			got := map[city.Cell]bool{}
			for cell := range laid(c) {
				got[cell] = true
			}
			assert.Equal(t, bridgeRoad(c), got)
		})

		t.Run("it should stand no lamps", func(t *testing.T) {
			assert.Empty(t, c.Lamps)
		})

		t.Run("it should plant street trees only beside the bridge's road", func(t *testing.T) {
			spur := bridgeRoad(c)
			var stray []city.Cell
			for _, tree := range streetTrees(c) {
				beside := false
				for _, n := range neighbours(tree.Cell) {
					beside = beside || spur[n]
				}
				if !beside {
					stray = append(stray, tree.Cell)
				}
			}
			assert.Empty(t, stray)
		})
	})

	t.Run("when two districts are connected", func(t *testing.T) {
		sessions := threeDistricts()
		for i := range sessions {
			sessions[i].State = state.NeedsYou
		}
		snap := snapshot(sessions...)
		snap.Roads = []state.Road{{From: "/p/a", To: "/p/c", Files: 2, Sessions: 1}}
		c := city.Build(snap, city.NewLayout())
		bare := city.Build(snapshot(sessions...), city.NewLayout())
		require.Len(t, c.Streets, 1)
		route := map[city.Cell]bool{}
		path := c.Streets[0].Path
		lots := map[city.Cell]bool{city.CellOf(path[0]): true, city.CellOf(path[len(path)-1]): true}
		for _, p := range path[1 : len(path)-1] {
			route[city.CellOf(p)] = true
		}
		road := laid(c)
		// The bridge's landing joins the bridge on its seaward side
		// (item 108), which is no road cell.
		seaward := map[city.Cell]int{}
		for _, b := range c.Bridges() {
			seaward[city.CellOf(b.Land)] = city.DirW
			if b.Far.X > b.Land.X {
				seaward[city.CellOf(b.Land)] = city.DirE
			}
		}

		t.Run("it should lay a road on exactly the corridor cells of its route, driveway to driveway, and the bridge's", func(t *testing.T) {
			got := map[city.Cell]bool{}
			for cell := range road {
				got[cell] = true
			}
			want := bridgeRoad(c)
			for cell := range route {
				want[cell] = true
			}
			assert.Equal(t, want, got)
		})

		t.Run("it should lay it only on corridors", func(t *testing.T) {
			grid := corridors(c)
			for cell := range road {
				assert.True(t, grid[cell], "cell %v", cell)
			}
		})

		t.Run("it should autotile each road cell against road neighbours, its lot and the bridge only", func(t *testing.T) {
			for cell, mask := range road {
				for dir, n := range neighbours(cell) {
					_, joined := road[n]
					joined = joined || lots[n] || seaward[cell] == dir
					assert.Equal(t, joined, mask&dir != 0, "cell %v towards %d", cell, dir)
				}
			}
		})

		t.Run("it should stand lamps only on laid road cells", func(t *testing.T) {
			for _, l := range c.Lamps {
				assert.Contains(t, road, l)
			}
		})

		t.Run("it should plant street trees only beside laid road cells", func(t *testing.T) {
			require.NotEmpty(t, streetTrees(c))
			for _, tree := range streetTrees(c) {
				beside := false
				for _, n := range neighbours(tree.Cell) {
					if _, ok := road[n]; ok {
						beside = true
					}
				}
				assert.True(t, beside, "tree on %v", tree.Cell)
			}
		})

		t.Run("it should keep the road hoverable with the pair's card", func(t *testing.T) {
			mid := c.Streets[0].Path[len(c.Streets[0].Path)/2]
			hit := c.Near(mid, 1)
			require.NotNil(t, hit.Road)
			assert.Equal(t, "a -> c", hit.Road.Card().Title)
		})

		t.Run("it should not move any district's block", func(t *testing.T) {
			for i, d := range c.Districts {
				assert.Equal(t, bare.Districts[i].Rect, d.Rect, d.Name)
			}
		})
	})

	t.Run("when both directions of a pair cross", func(t *testing.T) {
		snap := snapshot(threeDistricts()...)
		snap.Roads = []state.Road{
			{From: "/p/a", To: "/p/c", Files: 2, Sessions: 1, Active: 1},
			{From: "/p/c/.claude/worktrees/x", To: "/p/a", Messages: 1, Sessions: 1, Active: 1},
		}
		c := city.Build(snap, city.NewLayout())

		t.Run("it should make one road for the pair", func(t *testing.T) {
			assert.Len(t, c.Roads, 1)
		})

		t.Run("it should count every working traveller on it", func(t *testing.T) {
			require.Len(t, c.Roads, 1)
			assert.Equal(t, 2, c.Roads[0].Active)
		})
	})
}

func TestRoadGrowsOver(t *testing.T) {
	connected := func() state.Snapshot {
		snap := snapshot(threeDistricts()...)
		snap.Roads = []state.Road{{From: "/p/a", To: "/p/c", Files: 2, Sessions: 1}}
		return snap
	}
	scene := func(clock *time.Time, reduced bool) *city.Scene {
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		s.SetClock(func() time.Time { return *clock }, time.UTC)
		s.SetReducedMotion(reduced)
		s.SetSnapshot(connected())
		return s
	}

	t.Run("when a connection ceases", func(t *testing.T) {
		clock := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
		s := scene(&clock, false)
		was := laid(s.City())
		require.NotEmpty(t, was)
		s.SetSnapshot(snapshot(threeDistricts()...))

		t.Run("and less than the transition has passed", func(t *testing.T) {
			clock = clock.Add(city.OvergrowFor / 2)
			growing := s.Overgrowth()

			t.Run("it should draw every cell of the road no longer laid overgrowing", func(t *testing.T) {
				assert.Len(t, growing, len(was)-stillLaid(was, s.City()))
			})

			t.Run("it should be part way grown over", func(t *testing.T) {
				require.NotEmpty(t, growing)
				assert.InDelta(t, 0.5, growing[0].Progress, 0.01)
			})

			t.Run("it should keep the road's own shape", func(t *testing.T) {
				for _, g := range growing {
					assert.Equal(t, was[g.Cell], g.Mask, "cell %v", g.Cell)
				}
			})

			t.Run("it should stand no lamps on it", func(t *testing.T) {
				assert.Empty(t, s.City().Lamps)
			})

			t.Run("it should keep the map animating", func(t *testing.T) {
				assert.True(t, s.Animating())
			})
		})

		t.Run("and the transition has passed", func(t *testing.T) {
			clock = clock.Add(city.OvergrowFor)

			t.Run("it should draw belt", func(t *testing.T) {
				assert.Empty(t, s.Overgrowth())
			})
		})
	})

	t.Run("when a connection comes back while it is growing over", func(t *testing.T) {
		clock := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
		s := scene(&clock, false)
		s.SetSnapshot(snapshot(threeDistricts()...))
		clock = clock.Add(city.OvergrowFor / 2)
		s.SetSnapshot(connected())

		t.Run("it should end the transition", func(t *testing.T) {
			assert.Empty(t, s.Overgrowth())
		})

		t.Run("it should lay the road again", func(t *testing.T) {
			assert.NotEmpty(t, s.City().StreetCells)
		})
	})

	t.Run("when motion is reduced", func(t *testing.T) {
		clock := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
		s := scene(&clock, true)
		s.SetSnapshot(snapshot(threeDistricts()...))

		t.Run("it should cross-fade in the short time", func(t *testing.T) {
			clock = clock.Add(city.OvergrowFadeFor)
			assert.Empty(t, s.Overgrowth())
		})
	})
}

// bridgeRoad is the road laid only to reach a bridge (item 247).
func bridgeRoad(c *city.City) map[city.Cell]bool {
	cells := map[city.Cell]bool{}
	for _, cell := range c.BridgeRoads() {
		cells[cell] = true
	}
	return cells
}

// stillLaid is how many of a road's cells are laid in a city.
func stillLaid(was map[city.Cell]int, c *city.City) int {
	now := laid(c)
	n := 0
	for cell := range was {
		if _, ok := now[cell]; ok {
			n++
		}
	}
	return n
}
