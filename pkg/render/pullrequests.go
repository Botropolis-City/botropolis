package render

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
)

// Pull requests on the map: the inspector's car, the delivery truck, the
// bridge it leaves by, and the ribbon. Item 89.

const (
	// kitDelivery is the truck that takes a merged pull request's crate
	// away, and kitInspector the car parked outside while one is open.
	// Neither is a model the traffic drives, and the delivery truck is
	// not the depot's packing truck.
	kitDelivery  = "car-kit/delivery"
	kitInspector = "car-kit/suv"
	// kitVan carries one message between sessions, and nothing else.
	kitVan = "car-kit/van"
)

var colorRibbon = color.NRGBA{0xd8, 0x3a, 0x3a, 0xff}

// drawInspector draws an inspector's car at the kerb.
func (g *Game) drawInspector(screen *ebiten.Image, cam *city.Camera, b *city.Building, in city.Inspector) {
	var tint *ebiten.ColorScale
	if in.Fade < 1 {
		tint = &ebiten.ColorScale{}
		tint.ScaleAlpha(float32(in.Fade))
	}
	at, dir := inspectorPlace(in)
	r := g.kitScaled(screen, cam, kitInspector, vehicleTurn(dir), at, 0.8, tint)
	g.noteHit(r, city.Hit{Building: b, District: g.scene.City().DistrictOf(b), Inspector: b})
}

// drawDelivery draws a truck with its crate, fading out on the far side
// of the bridge, or where it stands at still.
func (g *Game) drawDelivery(screen *ebiten.Image, cam *city.Camera, bridge city.Bridge, d city.Delivery) {
	now := g.scene.Clock()
	p := d.Progress(now)
	fade := 1.0
	switch {
	case d.Fades:
		fade = 1 - p
	case p > 0.9:
		fade = (1 - p) / 0.1
	}
	tint := &ebiten.ColorScale{}
	tint.ScaleAlpha(float32(fade))
	at, dir := deliveryPlace(d, now)
	turn := vehicleTurn(dir)
	// On the bridge it rides at the deck's height (item 132).
	lift := bridge.Rise(at) * g.deckLift(cam)
	r := g.kitLifted(screen, cam, kitDelivery, turn, at, lift, tint)
	g.kitScaledLifted(screen, cam, crates[d.PR.Number%len(crates)], turn, at, 0.3, lift, tint)
	g.noteHit(r, city.Hit{Delivery: &d})
}

// drawMessage draws a message on its way: a van on the road, or a
// carriage on the group's line, fading in at its end at still.
func (g *Game) drawMessage(screen *ebiten.Image, cam *city.Camera, m city.Message) {
	now := g.scene.Clock()
	var tint *ebiten.ColorScale
	if m.Fades {
		tint = &ebiten.ColorScale{}
		tint.ScaleAlpha(float32(m.Progress(now)))
	}
	at, dir := messagePlace(m, now)
	turn := vehicleTurn(dir)
	model, shrink := kitVan, 0.9
	if m.Train {
		model = kitWagons[0]
		shrink = messageShrink(model)
	}
	r := g.kitScaled(screen, cam, model, turn, at, shrink, tint)
	g.noteHit(r, city.Hit{Message: &m})
}

// drawRibbon cuts a group's ribbon over a building: a ribbon that parts
// in the middle, and fireworks over the group's district when motion
// allows.
func (g *Game) drawRibbon(screen *ebiten.Image, cam *city.Camera, r city.Ribbon) {
	foot := cam.WorldToScreen(r.At)
	half := 14 * cam.Zoom
	gap := half * math.Min(1, r.P*3)
	alpha := uint8(255 * (1 - r.P))
	ribbon := colorRibbon
	ribbon.A = alpha
	vector.StrokeLine(screen, float32(foot.X-half), float32(foot.Y), float32(foot.X-gap/2), float32(foot.Y), float32(2*cam.Zoom), ribbon, true)
	vector.StrokeLine(screen, float32(foot.X+gap/2), float32(foot.Y), float32(foot.X+half), float32(foot.Y), float32(2*cam.Zoom), ribbon, true)
	if !r.Fireworks {
		return
	}
	over := city.RectAt(r.At.X-city.CellSize, r.At.Y-city.CellSize, 2*city.CellSize, 2*city.CellSize)
	for _, d := range g.scene.City().Districts {
		if d.Rect.Contains(r.At) {
			over = d.Rect
		}
	}
	green := g.theme.Palette.Merged
	for _, p := range sparks(r.At, over, r.P) {
		s := cam.WorldToScreen(p)
		lift := 30 * cam.Zoom * math.Sin(math.Pi*r.P)
		c := color.NRGBA{green.R, green.G, green.B, alpha}
		vector.FillCircle(screen, float32(s.X), float32(s.Y-lift), float32(1.5*cam.Zoom+0.5), c, true)
	}
}

// sparks is where the fireworks' sparks are p of the way through the
// show: a ring opening from at, held inside over, and gone at the end.
func sparks(at city.Point, over city.Rect, p float64) []city.Point {
	if p >= 1 || p < 0 {
		return nil
	}
	reach := math.Min(math.Min(at.X-over.Min.X, over.Max.X-at.X), math.Min(at.Y-over.Min.Y, over.Max.Y-at.Y))
	if reach <= 0 {
		return nil
	}
	const count = 12
	r := reach * 0.9 * (1 - (1-p)*(1-p))
	out := make([]city.Point, 0, count)
	for i := 0; i < count; i++ {
		a := 2 * math.Pi * float64(i) / count
		out = append(out, city.Point{X: at.X + r*math.Cos(a), Y: at.Y + r*math.Sin(a)})
	}
	return out
}
