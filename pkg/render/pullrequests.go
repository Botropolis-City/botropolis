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
	// bridgeStraight is the deck the bridge is laid with, east-west.
	bridgeStraight = "city-kit-roads/road-straight"
)

var (
	colorRibbon = color.NRGBA{0xd8, 0x3a, 0x3a, 0xff}
	colorPier   = color.NRGBA{0x5a, 0x5e, 0x66, 0xff}
)

// bridge lays the bridge's deck from the coast street out over the sea,
// one cell at a time, on piers. It is scenery: it is always there.
func (g *Game) bridge(screen *ebiten.Image, c *city.City, cam *city.Camera) {
	b := c.Bridge()
	if b.Land == b.Far {
		return
	}
	step := city.CellSize
	if b.Far.X < b.Land.X {
		step = -step
	}
	for x := b.Land.X + step; (step > 0 && x <= b.Far.X) || (step < 0 && x >= b.Far.X); x += step {
		at := city.Point{X: x, Y: b.Land.Y}
		foot := cam.WorldToScreen(at)
		pier := 3 * cam.Zoom
		vector.FillRect(screen, float32(foot.X-pier/2), float32(foot.Y), float32(pier), float32(6*cam.Zoom), colorPier, false)
		if g.kits == nil {
			g.rect(screen, cam, city.CellOf(at).Rect(), g.flat(colorStreet))
			continue
		}
		g.kitGround(screen, cam, bridgeStraight, 0, at, nil)
	}
}

// drawInspector draws an inspector's car at the kerb.
func (g *Game) drawInspector(screen *ebiten.Image, cam *city.Camera, b *city.Building, in city.Inspector) {
	var tint *ebiten.ColorScale
	if in.Fade < 1 {
		tint = &ebiten.ColorScale{}
		tint.ScaleAlpha(float32(in.Fade))
	}
	turn := 90
	if in.Leaving {
		turn = 270
	}
	r := g.kitScaled(screen, cam, kitInspector, turn, in.At, 0.8, tint)
	g.noteHit(r, city.Hit{Building: b, District: g.scene.City().DistrictOf(b), Inspector: b})
}

// drawDelivery draws a truck with its crate, fading out on the far side
// of the bridge, or where it stands at still.
func (g *Game) drawDelivery(screen *ebiten.Image, cam *city.Camera, d city.Delivery) {
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
	at := d.At(now)
	ahead := d.At(now.Add(100 * 1e6))
	turn := carTurn(ahead.Sub(at))
	r := g.kit(screen, cam, kitDelivery, turn, at, tint)
	g.kitScaled(screen, cam, crates[d.PR.Number%len(crates)], turn, at, 0.3, tint)
	g.noteHit(r, city.Hit{Delivery: &d})
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
