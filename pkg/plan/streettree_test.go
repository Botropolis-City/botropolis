package plan_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/plan"
)

// Bug 41. Aria: "several trees still land in concrete at the edge of
// properties". Not the jitter — TreeJitter is 0.3 against a half-cell
// of 0.5, so a park tree cannot leave its cell. A street tree was
// planted on a street cell and pushed KerbOffset 0.42 off centre, and
// 0.42 is also inside the cell, so it reached the far edge of the
// tarmac rather than a verge. The constant's comment described a verge
// that was never built.
//
// Bug 20's guard tested cells, and these trees are on a legal cell, so
// the guard has to be on where the tree actually ends up.

// lands is the cell a tree finally stands in, once its offset is taken
// into account.
func lands(t plan.Tree) plan.Cell {
	return plan.Cell{
		Col: t.Cell.Col + int(math.Floor(t.DX+0.5)),
		Row: t.Cell.Row + int(math.Floor(t.DY+0.5)),
	}
}

func TestStreetTreesStandClearOfTheCarriageway(t *testing.T) {
	p := plan.Make(plan.Input{Districts: live(9)}, plan.NewMemory())

	var street []plan.Tree
	for _, tree := range p.Trees {
		if tree.Kind == plan.StreetTree {
			street = append(street, tree)
		}
	}

	t.Run("when the city has laid its streets and planted them", func(t *testing.T) {
		require.NotEmpty(t, p.Streets, "no streets to plant")
		require.NotEmpty(t, street, "no street trees to check")

		t.Run("it should stand none of them on the carriageway", func(t *testing.T) {
			var on []plan.Cell
			for _, tree := range street {
				if p.IsStreet(lands(tree)) {
					on = append(on, lands(tree))
				}
			}
			assert.Empty(t, on, "%d of %d street trees stand in the road", len(on), len(street))
		})
	})
}
