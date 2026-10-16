package plan_test

import (
	"math"
	"testing"

	"botropolis.city/botropolis/pkg/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 85, slice 4: the trees that mean nothing are ground. They are
// clustered by a noise mask and spaced by Poisson-disk sampling over the
// parks, the belt and the corridors, rather than one to a park cell.

func groundTrees(p plan.Plan) []plan.Tree {
	var out []plan.Tree
	for _, t := range p.Trees {
		if t.Kind == plan.ParkTree {
			out = append(out, t)
		}
	}
	return out
}

func at(t plan.Tree) (float64, float64) {
	return float64(t.Cell.Col) + 0.5 + t.DX, float64(t.Cell.Row) + 0.5 + t.DY
}

func TestGroundTrees(t *testing.T) {
	in := plan.Input{Districts: live(4), Towers: 2, StorageRows: 2}
	p := plan.Make(in, plan.NewMemory())
	trees := groundTrees(p)
	require.NotEmpty(t, trees)

	t.Run("when the same plan is made twice", func(t *testing.T) {
		again := plan.Make(in, plan.NewMemory())

		t.Run("it should plant the same trees", func(t *testing.T) {
			assert.Equal(t, trees, groundTrees(again))
		})
	})

	t.Run("it should keep every two ground trees at least the spacing apart", func(t *testing.T) {
		for i := range trees {
			ax, ay := at(trees[i])
			for j := i + 1; j < len(trees); j++ {
				bx, by := at(trees[j])
				if d := math.Hypot(ax-bx, ay-by); d < plan.TreeSpacing-1e-9 {
					t.Fatalf("%v and %v are %.3f cells apart", trees[i], trees[j], d)
				}
			}
		}
	})

	t.Run("it should stand each tree inside its own cell", func(t *testing.T) {
		for _, tree := range trees {
			assert.True(t, math.Abs(tree.DX) <= 0.5 && math.Abs(tree.DY) <= 0.5, "%v", tree)
		}
	})

	t.Run("it should plant them only on parks, the belt and the corridors", func(t *testing.T) {
		rails := map[plan.Cell]bool{}
		for _, r := range p.Rails {
			rails[r] = true
		}
		for _, tree := range trees {
			c := tree.Cell
			open := p.IsStreet(c)
			for _, b := range append(append([]plan.Block(nil), p.Parks...), p.Belt...) {
				open = open || b.Contains(c)
			}
			blocked := rails[c] || p.Plaza.Contains(c) || p.Storage.Contains(c)
			for _, b := range p.Blocks {
				blocked = blocked || b.Contains(c)
			}
			for _, r := range p.River {
				blocked = blocked || r.Cell == c
			}
			assert.True(t, open && !blocked, "tree on %v", c)
		}
	})

	t.Run("it should cluster them, leaving some open ground bare", func(t *testing.T) {
		planted := map[plan.Cell]bool{}
		for _, tree := range trees {
			planted[tree.Cell] = true
		}
		bare := 0
		for _, b := range append(append([]plan.Block(nil), p.Parks...), p.Belt...) {
			for row := b.Min.Row; row < b.Min.Row+b.Rows; row++ {
				for col := b.Min.Col; col < b.Min.Col+b.Cols; col++ {
					if !planted[plan.Cell{Col: col, Row: row}] {
						bare++
					}
				}
			}
		}
		assert.Greater(t, bare, 0)
	})
}
