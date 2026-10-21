//go:build ebitenginedebug

package render

import "fmt"

// logLooks writes what the look cache did this frame beside
// Ebitengine's own command log, for item 194's frame test.
func (g *Game) logLooks() {
	if g.looks == nil {
		return
	}
	fmt.Printf("looks: hits=%d misses=%d bakes=%d bytes=%d\n", g.lookStats.hits, g.lookStats.misses, g.lookStats.bakes, g.looks.bytes())
}

// logStill writes what the standing city cost this frame, for item
// 193's frame test.
func (g *Game) logStill() {
	if g.noStill {
		return
	}
	s := g.still.stats
	fmt.Printf("still: composes=%d redos=%d overlays=%d rects=%d ground=%d direct=%d shifts=%d\n", s.composes, s.redos, s.overlays, s.rects, g.static.composes, s.direct, s.shifts+g.static.shifts)
}
