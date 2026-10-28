//go:build ebitenginedebug

package render

import (
	"fmt"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/state"
)

// logLooks writes what the look cache did this frame beside
// Ebitengine's own command log, for item 194's frame test.
func (g *Game) logLooks() {
	if g.looks == nil {
		return
	}
	fmt.Printf("looks: hits=%d misses=%d bakes=%d bytes=%d\n", g.lookStats.hits, g.lookStats.misses, g.lookStats.bakes, g.looks.bytes())
}

// logStill writes what the standing city cost this frame, and the tick
// it drew, for item 193's frame test and item 260's span of ticks.
func (g *Game) logStill() {
	if g.noStill {
		return
	}
	s := g.still.stats
	fmt.Printf("still: composes=%d redos=%d overlays=%d rects=%d ground=%d direct=%d shifts=%d tick=%d\n", s.composes, s.redos, s.overlays, s.rects, g.static.composes, s.direct, s.shifts+g.static.shifts, g.ticks)
}

// logOpened writes that a recording's city opened, on a read history,
// for a frame test to count from (item 202). One that reaches the
// ceiling unread fails rather than opening (item 240).
func (g *Game) logOpened() {
	if !g.capturing() {
		return
	}
	fmt.Printf("opened: on a read history\n")
}

// logMiss writes, for a look that missed, the frame and tick it missed
// on and which of its key's fields changed since its run's last frame,
// so a settled frame's work says whether the clock moved or a snapshot
// arrived (item 202).
func (g *Game) logMiss(id string, run step, key lookKey) {
	if g.looks == nil {
		return
	}
	fmt.Printf("look-miss: frame=%d tick=%d id=%s run=%d changed=%s\n", g.lookFrame, g.ticks, state.ShortID(id), run, lookChanged(g.looks.replaced, g.looks.hadReplaced, key))
}

// logReading writes the footer's reading line, for item 159's first
// launch test.
func (g *Game) logReading(line string) {
	if line != "" {
		fmt.Printf("reading: %s\n", line)
	}
}

// logOpening writes the opening panel's line, for item 159's first
// launch test.
func (g *Game) logOpening(line string) {
	fmt.Printf("opening: %s\n", line)
}

// openingFrame is where to keep one frame of the opening panel, for a
// person to judge: a recording starts only once the city has opened.
var openingFrame = os.Getenv("BOTROPOLIS_OPENING_FRAME")

// openingKept is whether that frame has been written; one city runs per
// process.
var openingKept bool

// filmingOpening is whether a frame of the opening panel is asked for
// and not yet kept, so even a headless city draws its panel (item 315).
func filmingOpening() bool { return openingFrame != "" && !openingKept }

// keepOpening writes the opening panel's frame to openingFrame once the
// city has waited two seconds, so the panel is up and its bar has moved.
func (g *Game) keepOpening(screen *ebiten.Image) {
	if openingFrame == "" || openingKept || time.Since(g.started) < 2*time.Second {
		return
	}
	openingKept = true
	if err := writePNG(openingFrame, frame(screen)); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

// framesWithoutMovers is whether the frame test asked for the city with
// no activity — no travellers, no building animating for its session —
// to compare two cities with the same movers (item 193; Aria,
// 2026-10-21: "Hold movers equal").
var framesWithoutMovers = os.Getenv("BOTROPOLIS_FRAME_MOVERS") == "none"

// lookCacheBytes is the look cache's cap, which the performance tests
// can set in MiB by BOTROPOLIS_LOOK_CACHE_CAP to run it under pressure
// (item 212).
func lookCacheBytes() int { return lookCacheCapFrom(os.Getenv("BOTROPOLIS_LOOK_CACHE_CAP")) }

// framesOpenUnread is whether a recording opens on its first snapshot
// instead of waiting for a read history. A recording waits so its heights
// do not change mid-shot (item 202), but item 159's first-launch test
// films the read itself, so it asks for this. Nothing else should.
var framesOpenUnread = os.Getenv("BOTROPOLIS_FRAME_OPEN_UNREAD") == "1"

// framesAllSighted is whether the frame test asked for a city that has
// captioned every kind already, so no first sighting comes down on a
// timer partway through what it counts (item 260).
var framesAllSighted = os.Getenv("BOTROPOLIS_FRAME_SIGHTED") == "all"
