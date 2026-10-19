package render

import (
	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// drawVoyage puts a tug on the river where its voyage has reached, bow
// the way it sails, and remembers it for the pointer.
func (g *Game) drawVoyage(screen *ebiten.Image, cam *city.Camera, v *city.Voyage, now float64) {
	at := v.At(g.scene.Clock())
	model := kitTugIn
	if v.Kind == city.Departure {
		model = kitTugOut
	}
	dir := city.Point{X: v.To.X - v.From.X, Y: v.To.Y - v.From.Y}
	r := g.kit(screen, cam, model, vehicleTurn(dir), at, nil)
	g.noteHit(r, city.Hit{Voyage: v})
}
