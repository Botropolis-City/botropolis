package plan

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

// nearBlockScan is nearBlock as it was, every block checked in turn: the
// oracle the indexed check must agree with.
func nearBlockScan(blocks map[string]Block, x, y float64) bool {
	for _, b := range blocks {
		dx := math.Max(0, math.Max(float64(b.Min.Col)-x, x-float64(b.Min.Col+b.Cols)))
		dy := math.Max(0, math.Max(float64(b.Min.Row)-y, y-float64(b.Min.Row+b.Rows)))
		if math.Hypot(dx, dy) < BlockClearance {
			return true
		}
	}
	return false
}

// Item 270: planting the ground checked every candidate tree against
// every block, so a city of 900 districts spent 2 s of its first build,
// on the game loop, in nearBlock.
func TestNearBlockAtScale(t *testing.T) {
	t.Run("when a plan of 900 blocks plants its ground", func(t *testing.T) {
		in := Input{}
		for i := range 900 {
			in.Districts = append(in.Districts, District{Root: fmt.Sprintf("/w/d%03d", i), Cols: 2 + i%3, Rows: 2 + i%2})
		}
		p := Make(in, nil)
		asked, checked := p.nearAsks, p.nearChecks

		t.Run("it should find a point near a block exactly where the block-by-block check does", func(t *testing.T) {
			var differ []string
			for y := float64(p.Bounds.Min.Row) - 2; y < float64(p.Bounds.Min.Row+p.Bounds.Rows)+2; y += 0.37 {
				for x := float64(p.Bounds.Min.Col) - 2; x < float64(p.Bounds.Min.Col+p.Bounds.Cols)+2; x += 0.37 {
					if p.nearBlock(x, y) != nearBlockScan(p.Blocks, x, y) {
						differ = append(differ, fmt.Sprintf("(%.2f, %.2f)", x, y))
					}
				}
			}
			assert.Empty(t, differ)
		})

		t.Run("it should test each point against only the blocks near it", func(t *testing.T) {
			assert.Less(t, float64(checked)/float64(max(asked, 1)), 20.0, "%d checks for %d points", checked, asked)
		})
	})
}
