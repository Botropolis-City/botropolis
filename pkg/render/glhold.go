package render

import "botropolis.city/botropolis/pkg/glhold"

func (g *Game) holdTheGL() {
	if g.holdGL != nil {
		g.holdGL()
		return
	}
	glhold.Hold(glhold.GL...)
}
