package render

import (
	"image/color"
	"path/filepath"
	"strings"

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
	lit := g.litGround(c)
	for _, sc := range cells {
		col := g.padColour(c.Night, fade)
		if groundTint(lit, sc.Cell) != nil {
			col = g.padColour(c.Night, 1-(1-fade)*recededAlpha)
		}
		g.padsOf(screen, c, cam, sc, col, flat)
	}
}

// recededAlpha is how much of a receded piece of ground still shows:
// dimmed's alpha.
const recededAlpha = 0.35

func (g *Game) padsOf(screen *ebiten.Image, c *city.City, cam *city.Camera, sc city.StreetCell, col color.NRGBA, flat bool) {
	for _, pad := range c.Pads([]city.StreetCell{sc}) {
		if flat {
			g.rect(screen, cam, pad, col)
			continue
		}
		g.poly(screen, cam, pad, col)
	}
}

// litGround is the street cells kept lit while a hovered car recedes
// the ground around its path, or nil when nothing recedes. Item 108.
func (g *Game) litGround(c *city.City) map[city.Cell]bool {
	if !g.lit.RecedesGround() {
		return nil
	}
	lit := map[city.Cell]bool{}
	for _, st := range c.Streets {
		if !g.lit.HasRoad(st.Road) {
			continue
		}
		for _, p := range st.Path[1 : len(st.Path)-1] {
			lit[city.CellOf(p)] = true
		}
	}
	return lit
}

// groundTint is the tint for a street cell: nil while it is lit, and
// the highlight's fade while a hovered car recedes it.
func groundTint(lit map[city.Cell]bool, cell city.Cell) *ebiten.ColorScale {
	if lit == nil || lit[cell] {
		return nil
	}
	return dimmed()
}

// carTarget is where the first car on the road between two districts,
// named "<from>-><to>", is now: a script's way to point at a car, which
// moves under it. Item 108.
func (g *Game) carTarget(c *city.City, cam *city.Camera, pair string) (city.Point, bool) {
	from, to, ok := strings.Cut(pair, "->")
	if !ok {
		return city.Point{}, false
	}
	for _, car := range g.cars(c, g.motion.ambient) {
		r := car.road
		if r != nil && r.From != nil && r.To != nil && filepath.Base(r.From.Root) == from && filepath.Base(r.To.Root) == to {
			return cam.WorldToScreen(car.at), true
		}
	}
	return city.Point{}, false
}

// carRoad is the road of the car under the pointer, or nil.
func carRoad(hover city.Hit) *city.RoadLine {
	if hover.Car == nil {
		return nil
	}
	return hover.Car.Road
}

// recededGround is a flat ground colour stepped back the way dimmed
// steps a sprite back.
func recededGround(c color.NRGBA) color.NRGBA {
	c.A = uint8(float64(c.A) * recededAlpha)
	return c
}
