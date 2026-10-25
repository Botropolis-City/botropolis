package city_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// Item 247: a road leads to every bridge, from the nearest road laid, or
// with none from the nearest driveway, so its ramp does not come down
// onto grass, the T is drawn, and the trucks drive on road.
func TestRoadToTheBridge(t *testing.T) {
	rootOf := func(i int) string { return fmt.Sprintf("/home/avesta/projects/d%02d", i) }
	chain := func(n int) [][2]int {
		var out [][2]int
		for i := 0; i < n; i++ {
			out = append(out, [2]int{i, i + 1})
		}
		return out
	}
	snapshotOf := func(districts int, roads [][2]int) state.Snapshot {
		var sessions []state.Session
		for i := 0; i < districts; i++ {
			s := session(fmt.Sprintf("s%02d", i), rootOf(i), state.Working)
			s.SubagentsInFlight = 0
			sessions = append(sessions, s)
		}
		var between []state.Road
		for _, r := range roads {
			between = append(between, state.Road{From: rootOf(r[0]), To: rootOf(r[1]), Files: 1, Messages: 1})
		}
		return state.Snapshot{At: now, Sessions: sessions, Roads: between}
	}
	roadOf := func(c *city.City) map[city.Cell]int {
		road := map[city.Cell]int{}
		for _, sc := range c.StreetCells {
			road[sc.Cell] = sc.Mask
		}
		return road
	}
	reachesDriveway := func(c *city.City, from city.Cell) bool {
		road := roadOf(c)
		driveways := map[city.Cell]bool{}
		for _, d := range c.Districts {
			if dw, ok := c.Driveway(d); ok {
				driveways[dw.Cell] = true
			}
		}
		seen := map[city.Cell]bool{from: true}
		queue := []city.Cell{from}
		for len(queue) > 0 {
			cell := queue[0]
			queue = queue[1:]
			if driveways[cell] {
				return true
			}
			for _, next := range []city.Cell{{Col: cell.Col + 1, Row: cell.Row}, {Col: cell.Col - 1, Row: cell.Row}, {Col: cell.Col, Row: cell.Row + 1}, {Col: cell.Col, Row: cell.Row - 1}} {
				if _, ok := road[next]; ok && !seen[next] {
					seen[next] = true
					queue = append(queue, next)
				}
			}
		}
		return false
	}
	seaward := func(b city.Bridge) int {
		if b.Far.X > b.Land.X {
			return city.DirE
		}
		return city.DirW
	}
	names := []string{"first", "second"}

	fixtures := []struct {
		name      string
		districts int
		roads     [][2]int
		bridges   int
	}{
		{"when two districts have no road between them", 2, nil, 1},
		{"when six districts have a road between two of them", 6, chain(1), 1},
		{"when six districts have a road whose far end is a long way from the bridge by the avenues", 6, [][2]int{{0, 5}}, 1},
		{"when seventeen districts have roads between them, so there are two bridges", 17, chain(6), 2},
	}
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			base := snapshotOf(f.districts, f.roads)
			c := city.Build(base, city.NewLayout())
			require.Len(t, c.Bridges(), f.bridges, "precondition: the fixture's bridges")
			road := roadOf(c)

			for i, bridge := range c.Bridges() {
				land := city.CellOf(bridge.Land)
				t.Run("and the "+names[i]+" bridge", func(t *testing.T) {
					t.Run("it should lay road on its landing", func(t *testing.T) {
						assert.Contains(t, road, land)
					})

					t.Run("it should join that road to a district's driveway by a run of road", func(t *testing.T) {
						assert.True(t, reachesDriveway(c, land))
					})

					t.Run("it should join the landing to the bridge", func(t *testing.T) {
						assert.NotZero(t, road[land]&seaward(bridge))
					})
				})
			}

			for _, d := range c.Districts {
				dw, ok := c.Driveway(d)
				if d.Depot || !ok {
					continue
				}
				if _, paved := road[dw.Cell]; !paved {
					continue
				}
				t.Run("and a PR merges in "+d.Name+", whose driveway is on road", func(t *testing.T) {
					s, path := mergeIn(t, base, d.Root)
					laid := roadOf(s.City())
					lot := -1
					for k, p := range path {
						if p == dw.Lot {
							lot = k
						}
					}
					require.NotEqual(t, -1, lot, "precondition: the truck leaves by its driveway")
					var off []city.Cell
					for _, p := range path[lot+1 : len(path)-1] {
						if _, ok := laid[city.CellOf(p)]; !ok {
							off = append(off, city.CellOf(p))
						}
					}

					t.Run("the truck's path from its driveway to the landing should run only on road", func(t *testing.T) {
						assert.Empty(t, off)
					})
				})
			}
		})
	}

	t.Run("when no road leaves any lot, so the bridge's road leaves one", func(t *testing.T) {
		c := city.Build(snapshotOf(2, nil), city.NewLayout())
		road := roadOf(c)
		var from []city.Driveway
		for _, d := range c.Districts {
			if dw, ok := c.Driveway(d); ok {
				if _, paved := road[dw.Cell]; paved {
					from = append(from, dw)
				}
			}
		}
		require.Len(t, from, 1, "precondition: the bridge's road starts at one driveway")
		dw := from[0]
		toward := city.DirS
		switch d := dw.Lot.Sub(dw.Cell.Center()); {
		case math.Abs(d.X) >= math.Abs(d.Y) && d.X > 0:
			toward = city.DirE
		case math.Abs(d.X) >= math.Abs(d.Y):
			toward = city.DirW
		case d.Y < 0:
			toward = city.DirN
		}

		t.Run("it should join its driveway to the lot", func(t *testing.T) {
			assert.NotZero(t, road[dw.Cell]&toward)
		})
	})

	t.Run("when the island grows rows north of the bridge, as a district grows", func(t *testing.T) {
		layout := city.NewLayout()
		cityOf := func(sessions int) *city.City {
			all := append(sessionsIn(cinders, sessions), session("m", mullet, state.Working))
			return city.Build(snapshot(all...), layout)
		}
		was := cityOf(1)
		grown := cityOf(8)
		require.Equal(t, was.Bridge().Land.Y-was.Plaza.Min.Y, grown.Bridge().Land.Y-grown.Plaza.Min.Y, "precondition: the bridge is anchored")

		t.Run("it should keep road on the anchored landing", func(t *testing.T) {
			assert.Contains(t, roadOf(grown), city.CellOf(grown.Bridge().Land))
		})
	})

	t.Run("when the sixteenth district arrives in a running scene", func(t *testing.T) {
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.SetSnapshot(snapshotOf(15, chain(6)))
		require.Len(t, s.City().Bridges(), 1, "precondition: one bridge at fifteen")
		s.SetSnapshot(snapshotOf(16, chain(6)))
		bridges := s.City().Bridges()
		require.Len(t, bridges, 2, "precondition: the second bridge is placed")

		t.Run("it should lay road on the second landing", func(t *testing.T) {
			assert.Contains(t, roadOf(s.City()), city.CellOf(bridges[1].Land))
		})
	})
}

// mergeIn merges a PR in the district at root and returns the scene and
// the truck's path.
func mergeIn(t *testing.T, base state.Snapshot, root string) (*city.Scene, []city.Point) {
	t.Helper()
	s := city.NewScene(city.NewLayout())
	s.Resize(800, 600)
	s.SetClock(func() time.Time { return now }, time.UTC)
	s.SetSnapshot(base)
	with := func(st string) state.Snapshot {
		snap := base
		snap.Sessions = append([]state.Session(nil), base.Sessions...)
		for k := range snap.Sessions {
			if snap.Sessions[k].CWD == root {
				snap.Sessions[k].PRs = []claude.PR{{Number: 1, URL: "https://github.com/o/r/pull/1", Repository: "o/r", State: st}}
				break
			}
		}
		return snap
	}
	s.SetSnapshot(with(claude.PROpen))
	s.SetSnapshot(with(claude.PRMerged))
	trucks := s.Deliveries()
	require.Len(t, trucks, 1, "precondition: one truck leaves")
	return s, trucks[0].Path
}
