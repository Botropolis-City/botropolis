//go:build !ebitenginedebug

package render

// logLooks reports nothing outside the debug build.
func (g *Game) logLooks() {}

// logStill reports nothing outside the debug build.
func (g *Game) logStill() {}

// logReading reports nothing outside the debug build.
func (g *Game) logReading(string) {}

// framesWithoutMovers is never so outside the debug build.
const framesWithoutMovers = false
