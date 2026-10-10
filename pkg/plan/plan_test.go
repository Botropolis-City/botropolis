package plan_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/plan"
)

func live(n int) []plan.District {
	var out []plan.District
	for i := 0; i < n; i++ {
		out = append(out, plan.District{Root: fmt.Sprintf("/p%d", i), Cols: 4, Rows: 4})
	}
	return out
}

func slotOf(p plan.Plan, root string) plan.Slot {
	return p.Slots[root]
}

func TestMake(t *testing.T) {
	t.Run("when one live district is planned", func(t *testing.T) {
		p := plan.Make(plan.Input{Districts: []plan.District{{Root: "/a", Cols: 5, Rows: 3}}}, plan.NewMemory())

		t.Run("it should put the plaza at the centre slot", func(t *testing.T) {
			assert.Equal(t, p.Plaza, p.BlockAt(plan.Slot{}))
		})

		t.Run("it should put the district on ring 1", func(t *testing.T) {
			assert.Equal(t, 1, slotOf(p, "/a").Ring())
		})

		t.Run("it should size the block to fit the district", func(t *testing.T) {
			block := p.Blocks["/a"]
			assert.True(t, block.Cols >= 5 && block.Rows >= 3, block)
		})

		t.Run("it should lay an avenue between the district and the plaza", func(t *testing.T) {
			block := p.Blocks["/a"]
			between := plan.Cell{Col: block.Min.Col - 1, Row: block.Min.Row}
			if slotOf(p, "/a").Col < 0 {
				between = plan.Cell{Col: block.Min.Col + block.Cols, Row: block.Min.Row}
			}
			if slotOf(p, "/a").Col == 0 {
				between = plan.Cell{Col: block.Min.Col, Row: block.Min.Row - 1}
				if slotOf(p, "/a").Row < 0 {
					between = plan.Cell{Col: block.Min.Col, Row: block.Min.Row + block.Rows}
				}
			}
			assert.True(t, p.IsStreet(between), between)
		})

		t.Run("it should remember the slot", func(t *testing.T) {
			mem := plan.NewMemory()
			plan.Make(plan.Input{Districts: []plan.District{{Root: "/a", Cols: 5, Rows: 3}}}, mem)
			assert.Contains(t, mem.Slots, "/a")
		})
	})

	t.Run("when nine live districts are planned", func(t *testing.T) {
		p := plan.Make(plan.Input{Districts: live(9)}, plan.NewMemory())

		t.Run("it should fill ring 1 first", func(t *testing.T) {
			ring1 := 0
			for _, s := range p.Slots {
				if s.Ring() == 1 {
					ring1++
				}
			}
			assert.Equal(t, 8, ring1)
		})

		t.Run("it should spill the ninth onto ring 2", func(t *testing.T) {
			assert.Equal(t, 2, slotOf(p, "/p8").Ring())
		})

		t.Run("it should give every district its own slot", func(t *testing.T) {
			seen := map[plan.Slot]bool{}
			for _, s := range p.Slots {
				seen[s] = true
			}
			assert.Len(t, seen, 9)
		})
	})

	t.Run("when a district remembers a free slot on ring 1", func(t *testing.T) {
		mem := plan.NewMemory()
		mem.Slots["/a"] = plan.Slot{Col: 1, Row: 1}
		p := plan.Make(plan.Input{Districts: []plan.District{{Root: "/a", Cols: 4, Rows: 4}, {Root: "/b", Cols: 4, Rows: 4}}}, mem)

		t.Run("it should keep it", func(t *testing.T) {
			assert.Equal(t, plan.Slot{Col: 1, Row: 1}, slotOf(p, "/a"))
		})

		t.Run("it should not give the same slot to the other district", func(t *testing.T) {
			assert.NotEqual(t, plan.Slot{Col: 1, Row: 1}, slotOf(p, "/b"))
		})
	})

	t.Run("when a district remembers a slot on ring 2 while ring 1 has room", func(t *testing.T) {
		mem := plan.NewMemory()
		mem.Slots["/a"] = plan.Slot{Col: 2, Row: 0}
		p := plan.Make(plan.Input{Districts: []plan.District{{Root: "/a", Cols: 4, Rows: 4}}}, mem)

		t.Run("it should keep the remembered slot rather than move", func(t *testing.T) {
			assert.Equal(t, plan.Slot{Col: 2, Row: 0}, slotOf(p, "/a"))
		})
	})

	t.Run("when two districts remember the same slot", func(t *testing.T) {
		mem := plan.NewMemory()
		mem.Slots["/a"] = plan.Slot{Col: 1, Row: 0}
		mem.Slots["/b"] = plan.Slot{Col: 1, Row: 0}
		p := plan.Make(plan.Input{Districts: []plan.District{{Root: "/a", Cols: 4, Rows: 4}, {Root: "/b", Cols: 4, Rows: 4}}}, mem)

		t.Run("it should keep the first by name and move the other to a free ring-1 slot", func(t *testing.T) {
			assert.Equal(t, plan.Slot{Col: 1, Row: 0}, slotOf(p, "/a"))
			assert.Equal(t, 1, slotOf(p, "/b").Ring())
			assert.NotEqual(t, slotOf(p, "/a"), slotOf(p, "/b"))
		})
	})

	t.Run("when slots on a ring go unused", func(t *testing.T) {
		p := plan.Make(plan.Input{Districts: live(1)}, plan.NewMemory())

		t.Run("it should fill them with parks", func(t *testing.T) {
			assert.Len(t, p.Parks, 7)
		})
	})

	t.Run("when the plan is bounded", func(t *testing.T) {
		p := plan.Make(plan.Input{Districts: live(1)}, plan.NewMemory())

		t.Run("it should start at the origin", func(t *testing.T) {
			assert.Equal(t, plan.Cell{}, p.Bounds.Min)
		})

		// The river widened to plan.RiverCols for the usage gauges: the
		// boats read across it, and at MinZoom a one-cell river put a
		// 25% step at 1.6 screen pixels.
		t.Run("it should run the river down the east edge", func(t *testing.T) {
			require.NotEmpty(t, p.River)
			east := p.Bounds.Min.Col + p.Bounds.Cols
			for _, r := range p.River {
				assert.GreaterOrEqual(t, r.Cell.Col, east-plan.RiverCols)
				assert.Less(t, r.Cell.Col, east)
			}
		})

		t.Run("it should run the river the full height", func(t *testing.T) {
			rows := map[int]bool{}
			for _, r := range p.River {
				rows[r.Cell.Row] = true
			}
			assert.Len(t, rows, p.Bounds.Rows)
		})

		t.Run("it should run it the full width at every row", func(t *testing.T) {
			assert.Len(t, p.River, p.Bounds.Rows*plan.RiverCols)
		})

		t.Run("it should wrap the west edge in park", func(t *testing.T) {
			assert.True(t, p.IsPark(plan.Cell{Col: 0, Row: p.Bounds.Rows / 2}))
		})

		t.Run("it should wrap the south edge in park", func(t *testing.T) {
			assert.True(t, p.IsPark(plan.Cell{Col: p.Bounds.Cols / 2, Row: p.Bounds.Rows - 1}))
		})

		t.Run("it should wrap the north edge in park", func(t *testing.T) {
			assert.True(t, p.IsPark(plan.Cell{Col: p.Bounds.Cols / 2, Row: 0}))
		})
	})

	t.Run("when there are parked sessions", func(t *testing.T) {
		p := plan.Make(plan.Input{Districts: live(1), StorageRows: 3}, plan.NewMemory())

		t.Run("it should place the storage block along the south", func(t *testing.T) {
			require.Positive(t, p.Storage.Rows)
			ring := p.BlockAt(plan.Slot{Col: 0, Row: 1})
			assert.Greater(t, p.Storage.Min.Row, ring.Min.Row+ring.Rows)
		})

		t.Run("it should span the ring's width", func(t *testing.T) {
			west, east := p.BlockAt(plan.Slot{Col: -1, Row: 0}), p.BlockAt(plan.Slot{Col: 1, Row: 0})
			assert.Equal(t, west.Min.Col, p.Storage.Min.Col)
			assert.Equal(t, east.Min.Col+east.Cols, p.Storage.Min.Col+p.Storage.Cols)
		})

		t.Run("it should be as tall as asked", func(t *testing.T) {
			assert.Equal(t, 3, p.Storage.Rows)
		})

		t.Run("it should keep a street between it and the ring", func(t *testing.T) {
			assert.True(t, p.IsStreet(plan.Cell{Col: p.Storage.Min.Col, Row: p.Storage.Min.Row - 1}))
		})
	})

	t.Run("when there are no parked sessions", func(t *testing.T) {
		p := plan.Make(plan.Input{Districts: live(1)}, plan.NewMemory())

		t.Run("it should have no storage block", func(t *testing.T) {
			assert.Zero(t, p.Storage.Rows)
		})
	})

	t.Run("when towers are given", func(t *testing.T) {
		p := plan.Make(plan.Input{Districts: live(1), Towers: 3}, plan.NewMemory())

		t.Run("it should line them along the north ridge", func(t *testing.T) {
			require.Len(t, p.Towers, 3)
			ring := p.BlockAt(plan.Slot{Col: 0, Row: -1})
			for _, c := range p.Towers {
				assert.Less(t, c.Row, ring.Min.Row)
			}
		})

		t.Run("it should space them a cell apart", func(t *testing.T) {
			assert.Equal(t, p.Towers[0].Col+2, p.Towers[1].Col)
		})
	})

	t.Run("when decorations are placed", func(t *testing.T) {
		p := plan.Make(plan.Input{Districts: live(1)}, plan.NewMemory())

		t.Run("it should put a fountain on the plaza's centre cell", func(t *testing.T) {
			assert.Equal(t, p.Plaza.Center(), p.Fountain)
		})

		t.Run("it should put a lamp at every crossing", func(t *testing.T) {
			for _, l := range p.Lamps {
				mask, _ := p.Street(l)
				assert.Equal(t, plan.DirN|plan.DirE|plan.DirS|plan.DirW, mask, l)
			}
			assert.NotEmpty(t, p.Lamps)
		})

		t.Run("it should plant a tree on every park cell", func(t *testing.T) {
			cells := 0
			for _, park := range p.Parks {
				cells += park.Cols * park.Rows
			}
			assert.GreaterOrEqual(t, len(p.Trees), cells)
		})
	})

	t.Run("when the rail loop is laid", func(t *testing.T) {
		p := plan.Make(plan.Input{Districts: live(3), Towers: 2, StorageRows: 2}, plan.NewMemory())
		// The loop rings the belt, west of the river. Its east edge used
		// to be written as cols-3, which equalled the belt's east side
		// only while the river was one cell wide — an arithmetic
		// coincidence, not a relationship. Widening the river for the
		// usage boats broke the coincidence and left the loop exactly
		// where it belongs, so the corner is derived from the river now
		// rather than from the map's width.
		cols, rows := p.Bounds.Cols, p.Bounds.Rows
		east := cols - plan.RiverCols - 2
		corners := []plan.Cell{{Col: 1, Row: 1}, {Col: east, Row: 1}, {Col: east, Row: rows - 2}, {Col: 1, Row: rows - 2}}

		t.Run("it should start at the north-west corner of the inner belt", func(t *testing.T) {
			require.NotEmpty(t, p.Rails)
			assert.Equal(t, corners[0], p.Rails[0])
		})

		t.Run("it should pass every corner", func(t *testing.T) {
			for _, c := range corners {
				assert.Contains(t, p.Rails, c)
			}
		})

		t.Run("it should be one closed loop of adjacent cells", func(t *testing.T) {
			perimeter := 2*(east-1) + 2*(rows-2-1)
			require.Len(t, p.Rails, perimeter)
			for i := range p.Rails {
				a, b := p.Rails[i], p.Rails[(i+1)%len(p.Rails)]
				assert.Equal(t, 1, abs(a.Col-b.Col)+abs(a.Row-b.Row), "step %d", i)
			}
		})

		t.Run("it should leave no tree on the rails", func(t *testing.T) {
			for _, r := range p.Rails {
				for _, tree := range p.Trees {
					assert.NotEqual(t, r, tree.Cell)
				}
			}
		})
	})

	t.Run("when trees are planted", func(t *testing.T) {
		p := plan.Make(plan.Input{Districts: live(2)}, plan.NewMemory())
		again := plan.Make(plan.Input{Districts: live(2)}, plan.NewMemory())
		require.NotEmpty(t, p.Trees)

		t.Run("it should set each park tree off its cell's centre within the jitter", func(t *testing.T) {
			for _, tree := range p.Trees {
				if tree.Kind != plan.ParkTree {
					continue
				}
				assert.LessOrEqual(t, math.Abs(tree.DX), plan.TreeJitter, tree)
				assert.LessOrEqual(t, math.Abs(tree.DY), plan.TreeJitter, tree)
			}
		})

		t.Run("it should stand every street tree out on the verge", func(t *testing.T) {
			for _, tree := range p.Trees {
				if tree.Kind != plan.StreetTree {
					continue
				}
				assert.Equal(t, plan.KerbOffset, math.Abs(tree.DX)+math.Abs(tree.DY), tree)
			}
		})

		t.Run("it should mix more than the two species the Suburban kit ships", func(t *testing.T) {
			seen := map[int]bool{}
			for _, tree := range p.Trees {
				if tree.Kind == plan.ParkTree {
					seen[tree.Variant] = true
				}
			}
			assert.Greater(t, len(seen), 2)
		})

		t.Run("it should keep every species inside the palette", func(t *testing.T) {
			limit := map[plan.TreeKind]int{plan.ParkTree: plan.ParkSpecies, plan.StreetTree: plan.StreetSpecies,
				plan.Bush: plan.BushSpecies, plan.Planter: 1}
			for _, tree := range p.Trees {
				assert.Less(t, tree.Variant, limit[tree.Kind], tree)
				assert.GreaterOrEqual(t, tree.Variant, 0, tree)
			}
		})

		t.Run("it should plant the same trees for the same plan", func(t *testing.T) {
			assert.Equal(t, p.Trees, again.Trees)
		})
	})

	t.Run("when streets are laid", func(t *testing.T) {
		p := plan.Make(plan.Input{Districts: live(1)}, plan.NewMemory())

		t.Run("it should join every avenue cell to a neighbour", func(t *testing.T) {
			for _, s := range p.Streets {
				assert.NotZero(t, s.Mask, s.Cell)
			}
		})

		t.Run("it should never join a cell to one that is not a street", func(t *testing.T) {
			for _, s := range p.Streets {
				if s.Mask&plan.DirE != 0 {
					assert.True(t, p.IsStreet(plan.Cell{Col: s.Cell.Col + 1, Row: s.Cell.Row}), s.Cell)
				}
			}
		})
	})
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func treesOf(p plan.Plan, kind plan.TreeKind) []plan.Tree {
	var out []plan.Tree
	for _, t := range p.Trees {
		if t.Kind == kind {
			out = append(out, t)
		}
	}
	return out
}

func TestPlanting(t *testing.T) {
	p := plan.Make(plan.Input{Districts: live(4), Towers: 3, StorageRows: 2}, plan.NewMemory())

	t.Run("when the avenues are planted", func(t *testing.T) {
		street := treesOf(p, plan.StreetTree)
		require.NotEmpty(t, street)

		// Bug 41 reversed this. Standing on an avenue cell was the
		// defect, not the invariant: a street cell is carriageway edge
		// to edge, so every tree that satisfied this assertion was
		// standing in the road. The tree now stands on the cell across
		// the kerb and leans back towards the street it lines.
		t.Run("it should stand every street tree off the carriageway", func(t *testing.T) {
			for _, tree := range street {
				assert.False(t, p.IsStreet(tree.Cell), tree)
			}
		})

		t.Run("it should still stand each of them beside the avenue it lines", func(t *testing.T) {
			for _, tree := range street {
				beside := false
				for _, n := range [4]plan.Cell{
					{Col: tree.Cell.Col + 1, Row: tree.Cell.Row},
					{Col: tree.Cell.Col - 1, Row: tree.Cell.Row},
					{Col: tree.Cell.Col, Row: tree.Cell.Row + 1},
					{Col: tree.Cell.Col, Row: tree.Cell.Row - 1},
				} {
					beside = beside || p.IsStreet(n)
				}
				assert.True(t, beside, tree)
			}
		})

		t.Run("it should plant a line at a fixed pitch", func(t *testing.T) {
			for _, tree := range street {
				assert.True(t, tree.Cell.Col%plan.StreetTreeSpacing == 0 || tree.Cell.Row%plan.StreetTreeSpacing == 0, tree)
			}
		})

		t.Run("it should plant one street in one species", func(t *testing.T) {
			byCol := map[int]map[int]bool{}
			for _, tree := range street {
				if tree.DX == 0 {
					continue
				}
				if byCol[tree.Cell.Col] == nil {
					byCol[tree.Cell.Col] = map[int]bool{}
				}
				byCol[tree.Cell.Col][tree.Variant] = true
			}
			for col, species := range byCol {
				assert.Len(t, species, 1, "column %d", col)
			}
		})
	})

	t.Run("when a park block is planted", func(t *testing.T) {
		require.NotEmpty(t, p.Parks)
		block := p.Parks[0]
		species := map[int]bool{}
		for _, tree := range treesOf(p, plan.ParkTree) {
			if block.Contains(tree.Cell) {
				species[tree.Variant] = true
			}
		}

		t.Run("it should be a grove of two to four species", func(t *testing.T) {
			assert.GreaterOrEqual(t, len(species), 2)
			assert.LessOrEqual(t, len(species), 4)
		})
	})

	t.Run("when the belt is planted", func(t *testing.T) {
		species := map[int]bool{}
		for _, tree := range treesOf(p, plan.ParkTree) {
			for _, b := range p.Belt {
				if b.Contains(tree.Cell) {
					species[tree.Variant] = true
				}
			}
		}

		t.Run("it should mix more species than a single park grove", func(t *testing.T) {
			assert.Greater(t, len(species), 4)
		})
	})

	t.Run("when the plaza is edged", func(t *testing.T) {
		edge := append(treesOf(p, plan.Bush), treesOf(p, plan.Planter)...)
		require.NotEmpty(t, edge)

		t.Run("it should keep every bush and planter on the plaza's rim", func(t *testing.T) {
			for _, tree := range edge {
				onRim := tree.Cell.Col == p.Plaza.Min.Col || tree.Cell.Col == p.Plaza.Min.Col+p.Plaza.Cols-1 ||
					tree.Cell.Row == p.Plaza.Min.Row || tree.Cell.Row == p.Plaza.Min.Row+p.Plaza.Rows-1
				assert.True(t, onRim, tree)
			}
		})

		t.Run("it should set both bushes and planters", func(t *testing.T) {
			assert.NotEmpty(t, treesOf(p, plan.Bush))
			assert.NotEmpty(t, treesOf(p, plan.Planter))
		})
	})

	t.Run("when the same plan is made twice", func(t *testing.T) {
		again := plan.Make(plan.Input{Districts: live(4), Towers: 3, StorageRows: 2}, plan.NewMemory())

		t.Run("it should grow the same garden", func(t *testing.T) {
			assert.Equal(t, p.Trees, again.Trees)
		})
	})
}
