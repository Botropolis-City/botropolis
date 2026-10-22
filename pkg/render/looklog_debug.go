//go:build ebitenginedebug

package render

import (
	"fmt"
	"os"
)

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

// logReading writes the footer's reading line, for item 159's first
// launch test.
func (g *Game) logReading(line string) {
	if line != "" {
		fmt.Printf("reading: %s\n", line)
	}
}

// framesWithoutMovers is whether the frame test asked for the city with
// no activity — no travellers, no building animating for its session —
// to compare two cities with the same movers (item 193; Aria,
// 2026-10-21: "Hold movers equal").
var framesWithoutMovers = os.Getenv("BOTROPOLIS_FRAME_MOVERS") == "none"
