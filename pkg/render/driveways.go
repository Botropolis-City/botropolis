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

// A fader draws what a hovered car does not keep onto a layer, and lays
// the layer over the screen at the highlight's fade before each thing
// kept at full strength, so the depth order holds. Item 108.
type fader struct {
	// target is the image the draw closures draw on; they capture the
	// variable, so pointing it at the layer redirects them.
	target      **ebiten.Image
	real, layer *ebiten.Image
	dirty       bool
	flush       func()
}

// newFader is a fader over a screen, its layer kept on the game and
// cleared as it is laid down.
func (g *Game) newFader(screen **ebiten.Image) *fader {
	real := *screen
	size := real.Bounds().Size()
	if g.carFadeLayer == nil || g.carFadeLayer.Bounds().Size() != size {
		if g.carFadeLayer != nil {
			g.carFadeLayer.Deallocate()
		}
		g.carFadeLayer = ebiten.NewImage(size.X, size.Y)
	}
	layer := g.carFadeLayer
	layer.Clear()
	return &fader{target: screen, real: real, layer: layer, flush: func() {
		op := &ebiten.DrawImageOptions{}
		op.ColorScale.ScaleAlpha(recededAlpha)
		real.DrawImage(layer, op)
		layer.Clear()
	}}
}

// faded draws onto the layer.
func (f *fader) faded(draw func()) func() {
	return func() {
		*f.target = f.layer
		draw()
		*f.target = f.real
		f.dirty = true
	}
}

// kept lays down what is faded so far, then draws on the screen.
func (f *fader) kept(draw func()) func() {
	return func() {
		f.done()
		draw()
	}
}

// fadeUnkept sends everything not kept to a fader's layer while a
// hovered car fades the city around it, and is nil otherwise.
func (g *Game) fadeUnkept(screen **ebiten.Image, items []drawable, airborne []func()) *fader {
	if !g.lit.RecedesGround() {
		return nil
	}
	f := g.newFader(screen)
	for i := range items {
		if items[i].kept {
			items[i].draw = f.kept(items[i].draw)
		} else {
			items[i].draw = f.faded(items[i].draw)
		}
	}
	for i := range airborne {
		airborne[i] = f.faded(airborne[i])
	}
	return f
}

// fadeAll draws something none of which a hovered car keeps, faded while
// it fades the city around it.
func (g *Game) fadeAll(screen **ebiten.Image, draw func()) {
	if !g.lit.RecedesGround() {
		draw()
		return
	}
	f := g.newFader(screen)
	f.faded(draw)()
	f.done()
}

// done lays down whatever is still on the layer; a nil fader has none.
func (f *fader) done() {
	if f != nil && f.dirty {
		f.flush()
		f.dirty = false
	}
}

// keeps is whether a district, and everything on it, stays at full
// strength under the highlight.
func (g *Game) keeps(d *city.District) bool { return !g.lit.FadesDistrict(d) }

// keepsRoad is the same for a road and its cars.
func (g *Game) keepsRoad(r *city.RoadLine) bool { return !g.lit.FadesRoad(r) }

// stationLit is whether a group-line station on a district is drawn
// apart from its line, at full strength over the faded line: one on a
// hovered car's districts (Aria, 2026-10-18).
func (g *Game) stationLit(d *city.District) bool {
	return g.lit.RecedesGround() && g.keeps(d)
}
