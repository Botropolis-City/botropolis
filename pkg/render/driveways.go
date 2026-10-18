package render

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// A driveway's pad is the strip of tarmac from the street into its lot,
// laid while its road is and growing over with it. Item 108.

// colorPad is the road kit's asphalt as it draws by day.
var colorPad = color.NRGBA{0x76, 0x7d, 0x97, 0xff}

// padColour is the pad's colour at a fade from 0, tarmac, to 1, gone.
func (g *Game) padColour(night bool, fade float64) color.NRGBA {
	col := colorPad
	if night {
		col = color.NRGBA{R: uint8(float64(col.R) * 0.64), G: uint8(float64(col.G) * 0.68), B: uint8(float64(col.B) * 0.71), A: col.A}
	}
	col = g.flat(col)
	col.A = uint8(float64(col.A) * (1 - fade))
	return col
}

// drivewayPads paves the pad off every driveway among the laid cells.
func (g *Game) drivewayPads(screen *ebiten.Image, c *city.City, cam *city.Camera, flat bool) {
	g.pads(screen, c, cam, c.StreetCells, 0, flat)
}

// overgrownPads fades the pad off every driveway growing over.
func (g *Game) overgrownPads(screen *ebiten.Image, c *city.City, cam *city.Camera, flat bool) {
	for _, o := range g.scene.Overgrowth() {
		g.pads(screen, c, cam, []city.StreetCell{{Cell: o.Cell, Mask: o.Mask}}, o.Progress, flat)
	}
}

func (g *Game) pads(screen *ebiten.Image, c *city.City, cam *city.Camera, cells []city.StreetCell, fade float64, flat bool) {
	col := g.padColour(c.Night, fade)
	for _, pad := range c.Pads(cells) {
		if flat {
			g.rect(screen, cam, pad, col)
			continue
		}
		g.poly(screen, cam, pad, col)
	}
}
