package render

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// A district being dragged to another slot: a ghost of its block held at
// the pointer, over the outline of the slot it would land on, drawn in
// the error colour when that slot will not take it. Item 85.

type ghost struct {
	block   city.Rect
	target  city.Rect
	refused bool
}

func districtGhost(drag city.Drag) ghost {
	r := drag.District.Rect
	block := city.RectAt(drag.At.X-r.Width()/2, drag.At.Y-r.Height()/2, r.Width(), r.Height())
	return ghost{block: block, target: drag.Target, refused: !drag.Droppable}
}

var colorGhost = color.NRGBA{0xff, 0xff, 0xff, 0x50}

// panHeld is the modifier that makes any drag pan, wherever it starts.
func panHeld() bool {
	return ebiten.IsKeyPressed(ebiten.KeyShift) || ebiten.IsKeyPressed(ebiten.KeyAlt)
}

func (g *Game) districtGhost(screen *ebiten.Image, cam *city.Camera) {
	drag, ok := g.scene.DistrictDrag()
	if !ok {
		return
	}
	gh := districtGhost(drag)
	outline := colorHighlight
	if gh.refused {
		outline = ui.DefaultPalette.Error
	}
	g.polyStroke(screen, cam, gh.target, 2, outline)
	g.poly(screen, cam, gh.block, colorGhost)
}
