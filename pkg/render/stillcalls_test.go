package render

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/colorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stillRuns is how many runs the standing city's record plays in: draws
// one after another from one source, through one blend, filter and
// colour matrix, at one mipmap level, are a run. A ring, a word and a
// fill each go down alone, so each is a run of its own.
func stillRuns(g *Game) int {
	runs, prev := 0, -1
	var prevCM *colorm.ColorM
	for _, e := range g.still.entries {
		if e.moves {
			continue
		}
		for i := e.from; i < e.to; i++ {
			c := &g.rec.cmds[i]
			if c.kind != cmdImage && c.kind != cmdMesh && c.kind != cmdColorM {
				runs, prev = runs+1, -1
				continue
			}
			var cm *colorm.ColorM
			if c.kind == cmdColorM {
				cm = &g.rec.cmOps[c.op].cm
			}
			k := g.rec.batchKey(i)
			if k != prev || (cm == nil) != (prevCM == nil) || cm != nil && !sameColorM(cm, prevCM) {
				runs++
			}
			prev, prevCM = k, cm
		}
	}
	return runs
}

// Item 328: Ebitengine takes each draw call's uniforms from a buffer
// its command queue keeps at the most any frame has asked of it, so a
// nightfall frame of tens of thousands of calls left each queue holding
// 8 MiB more, a queue at a time. The standing city goes down a run at a
// time instead of a draw at a time.
func TestStillDrawCalls(t *testing.T) {
	t.Run("when the standing city of 200 districts is composed whole at night", func(t *testing.T) {
		d := manyDistricts(t)
		require.NoError(t, d.Press("n"))
		require.NoError(t, d.Run(5))
		g := d.game
		require.True(t, g.scene.City().Night)
		require.NotNil(t, g.still.img)
		under := ebiten.NewImage(g.still.img.Bounds().Dx(), g.still.img.Bounds().Dy())
		t.Cleanup(under.Deallocate)
		runs := stillRuns(g)
		before := g.rec.calls
		g.composeStill(g.still.img.Bounds(), under)
		calls := g.rec.calls - before

		t.Run("it should make no more draw calls than its record has runs", func(t *testing.T) {
			assert.LessOrEqual(t, calls, runs)
		})
	})
}
