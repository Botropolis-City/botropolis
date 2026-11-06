package city_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 348, slice 2: a group's adjacent buildings in a district make a
// neighbourhood, one block of ground the place is known by.
func TestNeighbourhoodBlocks(t *testing.T) {
	// covers is whether the union of rects holds a point.
	covers := func(rects []city.Rect, p city.Point) bool {
		for _, r := range rects {
			if p.X >= r.Min.X && p.X <= r.Max.X && p.Y >= r.Min.Y && p.Y <= r.Max.Y {
				return true
			}
		}
		return false
	}
	districtOf := func(t *testing.T, c *city.City, root string) *city.District {
		t.Helper()
		for _, d := range c.Districts {
			if d.Root == root {
				return d
			}
		}
		require.FailNow(t, "no district "+root)
		return nil
	}
	ids := func(n city.Neighbourhood) []string {
		var out []string
		for _, b := range n.Buildings {
			out = append(out, b.Session.ID)
		}
		return out
	}
	dist := func(a, b city.Point) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }
	mid := func(a, b city.Point) city.Point { return city.Point{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2} }

	t.Run("when a group has three adjacent buildings in a district", func(t *testing.T) {
		// a, b, c fill the first row of a three-wide grid; n sits under a.
		c := city.Build(withGroups(inOrder(in("a", "g1"), in("b", "g1"), in("c", "g1"), loose("n"))), city.NewLayout())
		d := districtOf(t, c, cinders)
		hoods := c.Neighbourhoods(d)
		require.Len(t, hoods, 1)
		hood := hoods[0]
		a, b, n := buildingIn(t, c, "a"), buildingIn(t, c, "b"), buildingIn(t, c, "n")

		t.Run("it should give the district one neighbourhood for the group", func(t *testing.T) {
			assert.Equal(t, "g1", hood.Group.ID)
		})

		t.Run("it should hold the three buildings", func(t *testing.T) {
			assert.ElementsMatch(t, []string{"a", "b", "c"}, ids(hood))
		})

		t.Run("it should cover the gap between two adjacent lots", func(t *testing.T) {
			assert.True(t, covers(hood.Ground, mid(a.Rect.Center(), b.Rect.Center())))
		})

		t.Run("it should cover no lot outside the group", func(t *testing.T) {
			assert.False(t, covers(hood.Ground, n.Rect.Center()))
		})

		t.Run("it should not cover the gap between the group and an outsider", func(t *testing.T) {
			assert.False(t, covers(hood.Ground, mid(a.Rect.Center(), n.Rect.Center())))
		})
	})

	t.Run("when four of a group fill a square of lots", func(t *testing.T) {
		// a, b and n, m fill the first two rows' first two lots, with x
		// and y beside them: a b x / n m y.
		c := city.Build(withGroups(inOrder(in("a", "g1"), in("b", "g1"), loose("x"), in("n", "g1"), in("m", "g1"), loose("y"))), city.NewLayout())
		hood := c.Neighbourhoods(districtOf(t, c, cinders))
		require.Len(t, hood, 1)
		a, m := buildingIn(t, c, "a"), buildingIn(t, c, "m")

		t.Run("it should pave the crossing in the middle of the square", func(t *testing.T) {
			assert.True(t, covers(hood[0].Ground, mid(a.Rect.Center(), m.Rect.Center())))
		})
	})

	t.Run("when a group's buildings in a district are not adjacent", func(t *testing.T) {
		// a and c sit at either end of the first row, x between them:
		// c, a guess on a lot it already had, does not move.
		layout := city.NewLayout()
		city.Build(withGroups(inOrder(in("a", "g1"), loose("x"), loose("c"))), layout)
		c := city.Build(withGroups(inOrder(in("a", "g1"), loose("x"), in("c", "g1"))), layout)
		hoods := c.Neighbourhoods(districtOf(t, c, cinders))

		t.Run("it should make no neighbourhood of buildings that touch none of their own", func(t *testing.T) {
			assert.Empty(t, hoods)
		})
	})

	t.Run("when a group has one building in a district", func(t *testing.T) {
		c := city.Build(withGroups(inOrder(in("a", "g1"), loose("x"), loose("y"))), city.NewLayout())

		t.Run("it should make no neighbourhood", func(t *testing.T) {
			assert.Empty(t, c.Neighbourhoods(districtOf(t, c, cinders)))
		})
	})

	t.Run("when no session is in a group", func(t *testing.T) {
		c := city.Build(withGroups(inOrder(loose("x"), loose("y"), loose("z"))), city.NewLayout())

		t.Run("it should make no neighbourhood", func(t *testing.T) {
			assert.Empty(t, c.Neighbourhoods(districtOf(t, c, cinders)))
		})
	})

	t.Run("when a group spans two districts", func(t *testing.T) {
		other := func(id string) state.Session {
			s := session(id, mullet, state.Working)
			s.Group = "g1"
			return s
		}
		c := city.Build(withGroups(inOrder(in("a", "g1"), in("b", "g1"), other("p"), other("q"))), city.NewLayout())

		t.Run("it should make one neighbourhood in each", func(t *testing.T) {
			assert.Equal(t, []int{1, 1}, []int{len(c.Neighbourhoods(districtOf(t, c, cinders))), len(c.Neighbourhoods(districtOf(t, c, mullet)))})
		})
	})

	t.Run("when the neighbourhood's gate is placed", func(t *testing.T) {
		c := city.Build(withGroups(inOrder(in("a", "g1"), in("b", "g1"), loose("x"), loose("y"))), city.NewLayout())
		d := districtOf(t, c, cinders)
		hoods := c.Neighbourhoods(d)
		require.Len(t, hoods, 1)
		dw, ok := c.Driveway(d)
		require.True(t, ok, "the fixture should have a driveway")
		var corners []city.Point
		for _, b := range hoods[0].Buildings {
			r := b.Rect
			corners = append(corners, r.Min, city.Point{X: r.Max.X, Y: r.Min.Y}, city.Point{X: r.Min.X, Y: r.Max.Y}, r.Max)
		}
		nearest := corners[0]
		for _, p := range corners {
			if dist(p, dw.Lot) < dist(nearest, dw.Lot) {
				nearest = p
			}
		}

		t.Run("it should stand at the block's corner nearest the district's driveway", func(t *testing.T) {
			assert.Equal(t, nearest, hoods[0].Gate)
		})
	})
}

// Item 348, slice 2: a neighbourhood's kerb runs round the outside of
// its block only, never between its own lots.
func TestNeighbourhoodEdges(t *testing.T) {
	square := func(x, y float64) city.Rect { return city.RectAt(x, y, 10, 10) }
	onAny := func(edges [][2]city.Point, p city.Point) bool {
		for _, e := range edges {
			if math.Abs((e[1].X-e[0].X)*(p.Y-e[0].Y)-(e[1].Y-e[0].Y)*(p.X-e[0].X)) < 1e-9 &&
				p.X >= math.Min(e[0].X, e[1].X)-1e-9 && p.X <= math.Max(e[0].X, e[1].X)+1e-9 &&
				p.Y >= math.Min(e[0].Y, e[1].Y)-1e-9 && p.Y <= math.Max(e[0].Y, e[1].Y)+1e-9 {
				return true
			}
		}
		return false
	}

	t.Run("when the block is two lots and the gap between them", func(t *testing.T) {
		n := city.Neighbourhood{Ground: []city.Rect{square(0, 0), {Min: city.Point{X: 10, Y: 0}, Max: city.Point{X: 14, Y: 10}}, square(14, 0)}}
		edges := n.Edges()
		length := 0.0
		for _, e := range edges {
			length += math.Hypot(e[1].X-e[0].X, e[1].Y-e[0].Y)
		}

		t.Run("it should run the whole outline", func(t *testing.T) {
			assert.InDelta(t, 2*24+2*10, length, 1e-9)
		})

		t.Run("it should not run where a lot meets the gap", func(t *testing.T) {
			assert.False(t, onAny(edges, city.Point{X: 10, Y: 5}))
		})
	})
}
