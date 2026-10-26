package render

import (
	"image"
	"math"

	"botropolis.city/botropolis/pkg/assets"
	"botropolis.city/botropolis/pkg/city"
)

// Item 252, Aria 2026-10-25: "Raise the deck". Where a bridge crosses
// the river its deck stands clear of the tallest boat drawn, so its
// height is read off the boats' own sprites: a taller boat raises it.

// channelBoat is a boat that sails the river, and how long it is drawn,
// or 0 when it is drawn at its natural size.
type channelBoat struct {
	piece  string
	extent float64
}

// channelBoats is every boat drawn on the river: the usage boats, the
// refresh courier and the voyage tugs.
var channelBoats = []channelBoat{
	{kitGaugeLiner, gaugeHulls[city.GaugeBig].extent},
	{kitGaugeCargo, gaugeHulls[city.GaugeMedium].extent},
	{kitGaugeSail, gaugeHulls[city.GaugeSmall].extent},
	{kitCourier, courierExtent},
	{kitTugIn, 0},
	{kitTugOut, 0},
}

// flatChannel is the channel's clearance when there is no atlas to
// measure the boats against: twice the deck's.
const flatChannel = 2 * flatDeck

// drawnTop is the least height a sprite's highest pixel can stand at,
// in its own pixels: column by column, how far its topmost pixel is
// above the far edge of its base diamond, which is the highest any
// point on the ground under it can be drawn. On a plain box it is
// roofLift; it also sees a mast or a funnel that stands above the far
// corner's line, which roofLift cannot.
func drawnTop(page *image.NRGBA, s assets.KitSprite) float64 {
	w := float64(s.Rect.Dx())
	ax, ay := float64(s.Anchor.X), float64(s.Anchor.Y)
	top := 0.0
	for x := range s.Rect.Dx() {
		ground := ay - (w/4 - math.Abs(float64(x)+0.5-ax)/2)
		for y := range s.Rect.Dy() {
			if page.NRGBAAt(s.Rect.Min.X+x, s.Rect.Min.Y+y).A > 0 {
				top = math.Max(top, ground-(float64(y)+0.5))
				break
			}
		}
	}
	return top
}

// measureBoats measures the drawn top of every boat sprite on a page.
func (a *kitAtlas) measureBoats(index int, page *image.NRGBA) {
	if a.tops == nil {
		a.tops = map[spriteAt]float64{}
	}
	for _, boat := range channelBoats {
		for _, heading := range assets.Headings {
			if s, ok := a.Sprite(boat.piece, heading); ok && s.Page == index {
				a.tops[spriteAt{page: s.Page, at: s.Rect.Min}] = drawnTop(page, s)
			}
		}
	}
}

// boatTop is how high the tallest boat's drawn top stands on screen, at
// every turn it can sail at; 0 before its page is up.
func (g *Game) boatTop(cam *city.Camera) float64 {
	atlas := g.kitAtlas(cam)
	if atlas == nil {
		return 0
	}
	top := 0.0
	for _, boat := range channelBoats {
		for _, turn := range []int{0, 90, 180, 270} {
			s, ok := atlas.Sprite(boat.piece, cam.Heading-turn)
			if !ok {
				continue
			}
			scale := cam.Zoom / atlas.Zoom
			if boat.extent > 0 {
				scale *= kitShrink(boat.extent, math.Max(float64(s.Rect.Dx()), float64(s.Rect.Dy())), atlas.Zoom)
			}
			top = math.Max(top, atlas.tops[spriteAt{page: s.Page, at: s.Rect.Min}]*scale)
		}
	}
	return top
}

// channelLift is how high the road on a deck over the river stands:
// its girder's underside clear of the tallest boat's drawn top.
func (g *Game) channelLift(cam *city.Camera) float64 {
	top := g.boatTop(cam)
	under := top * (1 + headroom)
	if top == 0 {
		under = flatChannel * cam.Zoom
	}
	return math.Max(under, g.deckUnderside(cam)) + deckThickness*cam.Zoom
}

// heights is how high the road stands at each of a bridge's heights, on
// screen at this camera.
func (g *Game) heights(cam *city.Camera) func(city.Height) float64 {
	deck, channel := g.deckLift(cam), g.channelLift(cam)
	return func(h city.Height) float64 {
		switch h {
		case city.AtDeck:
			return deck
		case city.AtChannel:
			return channel
		}
		return 0
	}
}

// roadLift is how high the road on a bridge stands at a point, on
// screen.
func (g *Game) roadLift(cam *city.Camera, b city.Bridge, p city.Point) float64 {
	return b.Lift(p, g.heights(cam))
}
