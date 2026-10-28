package city

import (
	"container/heap"
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/state"
)

// linearWalk is walkCosting as it was before item 315: the cheapest
// entry found by scanning the whole frontier, the first of equal cost in
// the order entries were added.
func linearWalk(c *City, from, to map[Cell]bool, extra func(Cell) float64) []Cell {
	type entry struct {
		node routeNode
		cost float64
	}
	best := map[routeNode]float64{}
	prev := map[routeNode]routeNode{}
	nearest := math.MinInt
	for _, st := range c.plan.Streets {
		nearest = max(nearest, c.depth(toCell(st.Cell)))
	}
	var frontier []entry
	for cell := range from {
		frontier = append(frontier, entry{routeNode{cell, 0}, 0})
		best[routeNode{cell, 0}] = 0
	}
	sort.Slice(frontier, func(i, j int) bool { return lessCell(frontier[i].node.cell, frontier[j].node.cell) })
	steps := []struct {
		dc, dr, dir int
	}{{1, 0, DirE}, {-1, 0, DirW}, {0, 1, DirS}, {0, -1, DirN}}
	for len(frontier) > 0 {
		k := 0
		for i := range frontier {
			if frontier[i].cost < frontier[k].cost {
				k = i
			}
		}
		cur := frontier[k]
		frontier = append(frontier[:k], frontier[k+1:]...)
		if cur.cost > best[cur.node] {
			continue
		}
		if to[cur.node.cell] {
			return unwind(prev, cur.node)
		}
		for _, st := range steps {
			next := routeNode{Cell{cur.node.cell.Col + st.dc, cur.node.cell.Row + st.dr}, st.dir}
			if !c.plan.IsStreet(fromCell(next.cell)) {
				continue
			}
			cost := cur.cost + 1 + tieCost*float64(nearest-c.depth(next.cell))
			if extra != nil {
				cost += extra(next.cell)
			}
			if cur.node.dir != 0 && cur.node.dir != st.dir {
				cost += turnCost
			}
			if old, seen := best[next]; seen && old <= cost {
				continue
			}
			best[next] = cost
			prev[next] = cur.node
			frontier = append(frontier, entry{next, cost})
		}
	}
	return nil
}

// Item 315: walkCosting scanned its whole frontier for the cheapest
// entry, which cost the 600-district city 0.15 CPU-seconds of its
// opening. Every road is laid along its walks, so a faster search must
// walk exactly the same cells, ties and all.
func TestWalkCosting(t *testing.T) {
	at := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	cityOf := func(districts int) *City {
		var sessions []state.Session
		for i := range districts {
			cwd := fmt.Sprintf("/p/d%03d", i)
			sessions = append(sessions, state.Session{ID: cwd, CWD: cwd, State: state.Working, StartedAt: at})
		}
		return Build(state.Snapshot{At: at, Sessions: sessions}, NewLayout())
	}
	// cells picks n street cells of c at random.
	cells := func(c *City, r *rand.Rand, n int) map[Cell]bool {
		out := map[Cell]bool{}
		for range n {
			out[toCell(c.plan.Streets[r.IntN(len(c.plan.Streets))].Cell)] = true
		}
		return out
	}
	extras := []struct {
		name  string
		extra func(Cell) float64
	}{
		{"no extra cost", nil},
		{"an extra cost on some cells", func(cell Cell) float64 {
			if (cell.Col*7+cell.Row*3)%5 == 0 {
				return 2
			}
			return 0
		}},
	}

	for _, districts := range []int{6, 40, 150} {
		c := cityOf(districts)
		for _, x := range extras {
			t.Run(fmt.Sprintf("when walks are asked across a city of %d districts, with %s", districts, x.name), func(t *testing.T) {
				r := rand.New(rand.NewPCG(uint64(districts), 315))
				differ := 0
				for range 60 {
					from, to := cells(c, r, 1+r.IntN(3)), cells(c, r, 1+r.IntN(3))
					if fmt.Sprint(c.walkCosting(from, to, x.extra)) != fmt.Sprint(linearWalk(c, from, to, x.extra)) {
						differ++
					}
				}

				t.Run("it should walk exactly the cells the linear search walked", func(t *testing.T) {
					assert.Zero(t, differ, "walks that differed, of 60")
				})
			})
		}
	}

	t.Run("when no street reaches the target", func(t *testing.T) {
		c := cityOf(6)
		from := map[Cell]bool{toCell(c.plan.Streets[0].Cell): true}
		to := map[Cell]bool{{Col: -1000, Row: -1000}: true}

		t.Run("it should find no walk", func(t *testing.T) {
			assert.Nil(t, c.walkCosting(from, to, nil))
		})
	})
}

// The equivalence above cannot see the frontier's tie rule: a step
// toward the camera costs a millionth less, so walks across a real plan
// almost never cost exactly the same. The rule is pinned here instead.
func TestWalkFrontier(t *testing.T) {
	// taken adds a node for each cost in turn, numbered by when it was
	// added, and says in which order the frontier gives them back.
	taken := func(costs ...float64) []int {
		var f walkFrontier
		for i, cost := range costs {
			f.add(routeNode{Cell{Col: i}, 0}, cost)
		}
		var out []int
		for f.Len() > 0 {
			out = append(out, heap.Pop(&f).(walkEntry).node.cell.Col)
		}
		return out
	}

	t.Run("when entries that cost the same are added", func(t *testing.T) {
		t.Run("it should give them back in the order they were added", func(t *testing.T) {
			assert.Equal(t, []int{0, 1, 2, 3, 4}, taken(1, 1, 1, 1, 1))
		})
	})

	t.Run("when cheaper entries are added after dearer ones", func(t *testing.T) {
		t.Run("it should give back the cheapest first, and of equal cost the first added", func(t *testing.T) {
			assert.Equal(t, []int{3, 1, 4, 0, 2}, taken(3, 2, 3, 1, 2))
		})
	})
}
