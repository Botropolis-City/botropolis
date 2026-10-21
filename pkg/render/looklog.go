//go:build !ebitenginedebug

package render

// logLooks reports nothing outside the debug build.
func (g *Game) logLooks() {}
