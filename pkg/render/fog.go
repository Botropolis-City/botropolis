package render

import (
	"image"
	"image/color"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// The fog over the sea where the bridge goes out (item 239), from
// pkg/city's puffs: each a soft ellipse of one texture built here, a
// falloff and nothing drawn by anyone (design/art-sources.md), standing
// at its share of the deck's height. It stands still, so it is composed
// into the standing city, and it notes no hover target.

// fogPuffSide is the puff texture's side in texels.
const fogPuffSide = 64

var (
	colorFogDay   = color.NRGBA{0xee, 0xf2, 0xf6, 0xff}
	colorFogNight = color.NRGBA{0x5c, 0x6c, 0x84, 0xff}
)

// levelFog stands over the deck and what drives on it.
const levelFog = levelOnDeck + 1

// fogEllipse is a puff on screen: its middle and its full width and
// height.
type fogEllipse struct {
	Centre city.Point
	W, H   float64
	Alpha  float64
}

// fogEllipses is every puff on screen, back to front.
func (g *Game) fogEllipses(cam *city.Camera, puffs []city.FogPuff) []fogEllipse {
	top := g.deckLift(cam)
	d := cam.WorldToScreen(city.Point{X: city.CellSize}).Sub(cam.WorldToScreen(city.Point{}))
	cell := math.Hypot(d.X, d.Y)
	order := append([]city.FogPuff(nil), puffs...)
	sort.SliceStable(order, func(i, j int) bool { return cam.Depth(order[i].At) < cam.Depth(order[j].At) })
	out := make([]fogEllipse, 0, len(order))
	for _, p := range order {
		w := p.Radius * cell * 1.6
		out = append(out, fogEllipse{Centre: raised(cam, p.At, p.Lift*top), W: w, H: w * 0.42, Alpha: p.Alpha})
	}
	return out
}

// bridgesInSort is every bridge in the depth sort, each with its parts
// and its own fog, as its own drawable so each sorts by its own coast.
// Item 220.
//
// keyed, when set, makes each part's keyed draw from its own (item 212).
func bridgesInSort(cam *city.Camera, c *city.City, part func(i int, b city.Bridge, p bridgePart) func(), fog func(puffs []city.FogPuff) func(), keyed func(b city.Bridge, p bridgePart, draw func()) func()) []drawable {
	var items []drawable
	for i, b := range c.Bridges() {
		from := len(items)
		items = append(items, bridgeItems(cam, b, func(p bridgePart) func() { return part(i, b, p) })...)
		if keyed != nil {
			parts := bridgeParts(b)
			for j := range min(len(parts), len(items)-from) {
				items[from+j].keyed = keyed(b, parts[j], items[from+j].draw)
			}
		}
		puffs := c.FogOf(b)
		if item, ok := fogItem(cam, puffs, fog(puffs)); ok {
			items = append(items, item)
		}
	}
	return items
}

// fogItem is the fog in the depth sort: one drawable over every puff's
// ground, a level above the trucks on the deck.
func fogItem(cam *city.Camera, puffs []city.FogPuff, draw func()) (drawable, bool) {
	if len(puffs) == 0 {
		return drawable{}, false
	}
	r := city.Rect{Min: puffs[0].At, Max: puffs[0].At}
	for _, p := range puffs {
		r.Min.X, r.Min.Y = min(r.Min.X, p.At.X), min(r.Min.Y, p.At.Y)
		r.Max.X, r.Max.Y = max(r.Max.X, p.At.X), max(r.Max.Y, p.At.Y)
	}
	item := footprintAt(cam, r, draw)
	item.level = levelFog
	return item, true
}

// fogImage is the puff: white, falling off from its middle as a
// Gaussian clipped to zero at its rim, premultiplied.
func (g *Game) fogImage() *ebiten.Image {
	if g.fogPuff != nil {
		return g.fogPuff
	}
	pix := make([]byte, 4*fogPuffSide*fogPuffSide)
	rim := math.Exp(-3.2)
	for y := range fogPuffSide {
		for x := range fogPuffSide {
			dx := (float64(x)+0.5)/fogPuffSide*2 - 1
			dy := (float64(y)+0.5)/fogPuffSide*2 - 1
			a := math.Max(0, math.Exp(-3.2*(dx*dx+dy*dy))-rim) / (1 - rim)
			v := uint8(math.Round(255 * a))
			i := 4 * (y*fogPuffSide + x)
			pix[i], pix[i+1], pix[i+2], pix[i+3] = v, v, v, v
		}
	}
	g.fogPuff = ebiten.NewImageWithOptions(image.Rect(0, 0, fogPuffSide, fogPuffSide), nil)
	g.fogPuff.WritePixels(pix)
	return g.fogPuff
}

// drawFog draws every puff, tinted for the light.
func (g *Game) drawFog(screen *ebiten.Image, cam *city.Camera, puffs []city.FogPuff, night bool) {
	img := g.fogImage()
	tint, strength := colorFogDay, 1.0
	if night {
		tint, strength = colorFogNight, 0.9
	}
	for _, e := range g.fogEllipses(cam, puffs) {
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		op.GeoM.Scale(e.W/fogPuffSide, e.H/fogPuffSide)
		op.GeoM.Translate(e.Centre.X-e.W/2, e.Centre.Y-e.H/2)
		a := float32(e.Alpha * strength)
		op.ColorScale.Scale(float32(tint.R)/255*a, float32(tint.G)/255*a, float32(tint.B)/255*a, a)
		g.recorder().Image(screen, img, op)
	}
}
