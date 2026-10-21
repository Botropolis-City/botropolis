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
