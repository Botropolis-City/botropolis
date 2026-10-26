package render

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

var (
	colorGroundNight = color.NRGBA{0x1e, 0x2b, 0x1c, 0xff}
	colorKerb        = color.NRGBA{0x0c, 0x0e, 0x12, 0xff}
	colorMapDistHi   = color.NRGBA{0x3a, 0x40, 0x4c, 0xff}
	colorMapLit      = color.NRGBA{0x55, 0x9a, 0xe0, 0xff}
	colorMapIdle     = color.NRGBA{0x3a, 0x44, 0x58, 0xff}
	colorMapNeedsYou = colorNeedsYou
	colorMapUnatt    = color.NRGBA{0x8a, 0x6c, 0xd8, 0xff}
	colorMapResting  = color.NRGBA{0x44, 0x42, 0x48, 0xff}
	colorMapVacant   = color.NRGBA{0x4e, 0x48, 0x3c, 0xff}
	colorVacant      = color.NRGBA{0x6b, 0x62, 0x50, 0xff}
	colorVacantDim   = color.NRGBA{0x3a, 0x36, 0x2e, 0xff}
	colorMinimapView = color.NRGBA{0xff, 0xff, 0xff, 0xc0}
	colorWater       = color.NRGBA{0x5a, 0xa8, 0xd0, 0xff}
	colorWaterLight  = color.NRGBA{0xc8, 0xe8, 0xf8, 0xff}
	colorWaterNight  = color.NRGBA{0x28, 0x48, 0x68, 0xff}
	colorVoid        = color.NRGBA{0x12, 0x14, 0x1a, 0xff}
	colorMapPark     = color.NRGBA{0x2f, 0x4a, 0x2a, 0xff}
	colorMapPlaza    = color.NRGBA{0x5a, 0x57, 0x50, 0xff}
	colorLampGlow    = color.NRGBA{0xff, 0xd0, 0x70, 0x50}
	colorStreet      = color.NRGBA{0x4a, 0x4c, 0x52, 0xff}
	colorHighlight   = color.NRGBA{0xff, 0xff, 0xff, 0xa0}
)

// floorLabel writes fixed-size text on a plate so it reads on any ground;
// at is where the text starts, the plate wraps it.
func (g *Game) floorLabel(screen *ebiten.Image, at city.Point, s string, c color.NRGBA) {
	g.plate(screen, g.floorPlate(at, s), s, c)
}

// floorPlate is where floorLabel puts its plate.
func (g *Game) floorPlate(at city.Point, s string) ui.Plate {
	pad := g.theme.Grid() / 2
	return ui.LayoutPlate(g.theme, s, city.Point{X: at.X - pad, Y: at.Y - g.theme.Grid()/4}, ui.Small, g.faces.Measure)
}

// blockColor is a building's map colour: the state alone, saturated enough
// to tell apart at a glance.
func blockColor(b *city.Building, seconds float64) color.NRGBA {
	switch {
	case b.Blinds:
		return colorMapResting
	case b.Vacant:
		return colorMapVacant
	case b.Pulse:
		return pulse(colorMapNeedsYou, seconds)
	case b.Session.State == state.Waiting:
		return ui.DefaultPalette.Waiting
	case b.Session.State == state.Unattended:
		return colorMapUnatt
	case b.Lit:
		return colorMapLit
	}
	return colorMapIdle
}

// minimap draws the whole city in a corner with the viewport marked, only
// when the viewport does not already show all of it.
func (g *Game) minimap(screen *ebiten.Image, box city.Rect) {
	th := g.theme
	m := g.scene.Minimap(box.Inset(th.Grid()))
	if m.Scale == 0 {
		return
	}
	c := g.scene.City()
	all := m.ProjectRect(c.Bounds())
	if m.View.Min.X <= all.Min.X && m.View.Min.Y <= all.Min.Y && m.View.Max.X >= all.Max.X && m.View.Max.Y >= all.Max.Y {
		return
	}
	g.roundPanel(screen, box)
	g.minimapMap(screen, box, m, c, all)
	view := city.Rect{
		Min: city.Point{X: math.Max(box.Min.X, m.View.Min.X), Y: math.Max(box.Min.Y, m.View.Min.Y)},
		Max: city.Point{X: math.Min(box.Max.X, m.View.Max.X), Y: math.Min(box.Max.Y, m.View.Max.Y)},
	}
	g.paint.BoxStroke(screen, float32(view.Min.X), float32(view.Min.Y), float32(view.Width()), float32(view.Height()), 1, colorMinimapView)
}
