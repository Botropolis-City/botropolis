package render

import (
	"image/color"
	"math"

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

// districtGhosts is a ghost for every district carried, the held one at
// the pointer and the rest placed round it as the group is shaped, each
// over the slot it would land on (item 283).
func districtGhosts(drag city.Drag) []ghost {
	held := districtGhost(drag)
	if len(drag.Members) == 0 {
		return []ghost{held}
	}
	ghosts := make([]ghost, len(drag.Members))
	for i, d := range drag.Members {
		off := d.Rect.Min.Sub(drag.District.Rect.Min)
		ghosts[i] = ghost{
			block:   city.RectAt(held.block.Min.X+off.X, held.block.Min.Y+off.Y, d.Rect.Width(), d.Rect.Height()),
			target:  drag.Target,
			refused: !drag.Droppable,
		}
		if i < len(drag.Targets) {
			ghosts[i].target = drag.Targets[i]
		}
	}
	return ghosts
}

var colorGhost = color.NRGBA{0xff, 0xff, 0xff, 0x50}

// panHeld is the modifier that makes any drag pan, wherever it starts.
func panHeld() bool {
	return ebiten.IsKeyPressed(ebiten.KeyShift) || ebiten.IsKeyPressed(ebiten.KeyAlt)
}

func (g *Game) districtGhost(screen *ebiten.Image, cam *city.Camera) {
	g.districtSelection(screen, cam)
	drag, ok := g.scene.DistrictDrag()
	if !ok {
		return
	}
	for _, gh := range districtGhosts(drag) {
		outline := colorHighlight
		if gh.refused {
			outline = ui.DefaultPalette.Error
		}
		g.polyStroke(screen, cam, gh.target, 2, outline)
		g.poly(screen, cam, gh.block, colorGhost)
	}
}

// districtSelection outlines the selected districts, and the box being
// drawn to select more (item 283).
func (g *Game) districtSelection(screen *ebiten.Image, cam *city.Camera) {
	accent := g.theme.Palette.Accent
	tint := accent
	tint.A = 0x38
	for _, d := range g.scene.City().Districts {
		if g.scene.DistrictSelected(d.Root) {
			g.poly(screen, cam, d.Rect, tint)
			g.polyStroke(screen, cam, d.Rect, 3, accent)
		}
	}
	from, to, ok := g.selectBox()
	if !ok {
		return
	}
	x, y := math.Min(from.X, to.X), math.Min(from.Y, to.Y)
	w, h := math.Abs(to.X-from.X), math.Abs(to.Y-from.Y)
	g.paint.BoxStroke(screen, float32(x), float32(y), float32(w), float32(h), 1, colorHighlight)
}
