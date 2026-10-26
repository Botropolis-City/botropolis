//go:build !ebitenginedebug

package render

import "github.com/hajimehoshi/ebiten/v2"

// logLooks reports nothing outside the debug build.
func (g *Game) logLooks() {}

// logStill reports nothing outside the debug build.
func (g *Game) logStill() {}

// logOpened reports nothing outside the debug build.
func (g *Game) logOpened() {}

// logMiss reports nothing outside the debug build.
func (g *Game) logMiss(string, step, lookKey) {}

// logReading reports nothing outside the debug build.
func (g *Game) logReading(string) {}

// logOpening reports nothing outside the debug build.
func (g *Game) logOpening(string) {}

// keepOpening keeps no frame outside the debug build.
func (g *Game) keepOpening(*ebiten.Image) {}

// framesWithoutMovers is never so outside the debug build.
const framesWithoutMovers = false

// framesOpenUnread is never so outside the debug build: a recording
// always waits for a read history.
const framesOpenUnread = false

// framesAllSighted is never so outside the debug build: a first
// sighting is always captioned.
const framesAllSighted = false
