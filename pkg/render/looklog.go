//go:build !ebitenginedebug

package render

// logLooks reports nothing outside the debug build.
func (g *Game) logLooks() {}

// logStill reports nothing outside the debug build.
func (g *Game) logStill() {}

// framesWithoutMovers is never so outside the debug build.
const framesWithoutMovers = false
