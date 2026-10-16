package render

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// The island and the sea round it, painted from pkg/city's model of the
// ground. It is painted once per plan into a texture in world space, a
// texel to shoreTexel world units, and drawn under the plan through the
// camera's own affine projection, so the coast is soft and costs one
// draw a frame. Item 85.

const shoreTexel = 4.0

// shorePalette is the island's colours by day or by night.
type shorePalette struct {
	land, river, sand, shallows, sea, farShore color.NRGBA
}

var (
	dayShore = shorePalette{
		land: colorKitGrass, river: colorKitWater,
		sand:     color.NRGBA{0xd9, 0xcb, 0x9a, 0xff},
		shallows: color.NRGBA{0x8c, 0xc4, 0xd8, 0xff},
		sea:      color.NRGBA{0x4f, 0x8c, 0xbd, 0xff},
		farShore: color.NRGBA{0x9f, 0xb8, 0xa4, 0xff},
	}
	nightShore = shorePalette{
		land: ui.DefaultPalette.GroundNight, river: colorWaterNight,
		sand:     color.NRGBA{0x4e, 0x4c, 0x40, 0xff},
		shallows: color.NRGBA{0x30, 0x52, 0x70, 0xff},
		sea:      color.NRGBA{0x1e, 0x36, 0x52, 0xff},
		farShore: color.NRGBA{0x2c, 0x3c, 0x40, 0xff},
	}
)

// shorePaletteFor is the island's palette for a view: the top-down map
// keeps its own darker ground and water, and the island round it takes
// them too, so the plan does not read as a rectangle laid on it.
func shorePaletteFor(flat, night bool) shorePalette {
	p := dayShore
	if night {
		p = nightShore
	}
	if flat {
		p.land, p.river = colorGround, colorWater
		if night {
			p.land, p.river = colorGroundNight, colorWaterNight
		}
	}
	return p
}

// shorePaint is the colour of a kind of ground by day, t across its band.
func shorePaint(g city.Ground, t float64) color.NRGBA { return dayShore.paint(g, t) }

func (s shorePalette) paint(g city.Ground, t float64) color.NRGBA {
	switch g {
	case city.GroundSand:
		return mix(s.land, s.sand, 0.65+0.35*t)
	case city.GroundShallows:
		return mix(s.shallows, s.sea, t)
	case city.GroundFarShore:
		return mix(s.sea, s.farShore, 0.15+0.4*t)
	case city.GroundRiver:
		// Its own blue in the plan, fading to the shallows' at the coast.
		return mix(s.river, s.shallows, t)
	case city.GroundSea:
		return s.sea
	}
	return s.land
}

func mix(a, b color.NRGBA, t float64) color.NRGBA {
	t = math.Max(0, math.Min(1, t))
	l := func(x, y uint8) uint8 { return uint8(math.Round(float64(x) + (float64(y)-float64(x))*t)) }
	return color.NRGBA{l(a.R, b.R), l(a.G, b.G), l(a.B, b.B), 0xff}
}

type shoreLayer struct {
	island city.Island
	night  bool
	flat   bool
	// receded is the top-down view stepping its ground back for a view.
	receded bool
	sea     city.Rect
	img     *ebiten.Image
}

func (g *Game) shoreImage(c *city.City, flat bool) *shoreLayer {
	island := c.Island()
	receded := flat && g.viewing()
	if l := g.shoreLayer; l != nil && l.island == island && l.night == c.Night && l.flat == flat && l.receded == receded {
		return l
	}
	palette := shorePaletteFor(flat, c.Night)
	if flat {
		// The top-down map steps its ground back while a view is on, and
		// the island steps back with it.
		for _, col := range []*color.NRGBA{&palette.land, &palette.river, &palette.sand, &palette.shallows, &palette.sea, &palette.farShore} {
			*col = g.flat(*col)
		}
	}
	sea := island.Sea()
	w, h := int(sea.Width()/shoreTexel), int(sea.Height()/shoreTexel)
	pix := make([]byte, 4*w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			kind, t := island.At(city.Point{X: sea.Min.X + (float64(x)+0.5)*shoreTexel, Y: sea.Min.Y + (float64(y)+0.5)*shoreTexel})
			col := palette.paint(kind, t)
			i := 4 * (y*w + x)
			pix[i], pix[i+1], pix[i+2], pix[i+3] = col.R, col.G, col.B, 0xff
		}
	}
	img := ebiten.NewImage(w, h)
	img.WritePixels(pix)
	if g.shoreLayer != nil {
		g.shoreLayer.img.Deallocate()
	}
	g.shoreLayer = &shoreLayer{island: island, night: c.Night, flat: flat, receded: receded, sea: sea, img: img}
	return g.shoreLayer
}

// shore draws the island's ground and the sea under the plan.
func (g *Game) shore(screen *ebiten.Image, cam *city.Camera, c *city.City) {
	if c.Bounds().Area() == 0 {
		return
	}
	s := g.shoreImage(c, cam.Projection != city.Isometric)
	project := func(p city.Point) city.Point { return cam.Project(p).Add(cam.Offset).Scale(cam.Zoom) }
	o := project(s.sea.Min)
	ex := project(city.Point{X: s.sea.Min.X + shoreTexel, Y: s.sea.Min.Y}).Sub(o)
	ey := project(city.Point{X: s.sea.Min.X, Y: s.sea.Min.Y + shoreTexel}).Sub(o)
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.SetElement(0, 0, ex.X)
	op.GeoM.SetElement(1, 0, ex.Y)
	op.GeoM.SetElement(0, 1, ey.X)
	op.GeoM.SetElement(1, 1, ey.Y)
	op.GeoM.SetElement(0, 2, o.X)
	op.GeoM.SetElement(1, 2, o.Y)
	screen.DrawImage(s.img, op)
}
