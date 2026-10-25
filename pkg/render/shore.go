package render

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// The island and the sea round it, painted from pkg/city's model of the
// ground. It is painted once per plan into a texture in world space, a
// texel to shoreTexelFor world units, and drawn under the plan through
// the camera's own affine projection, so the coast is soft and costs one
// draw a frame. Item 85.

// shoreTexel is the finest the shore is painted, in world units a texel.
const shoreTexel = 4.0

// shoreTexelSide is the most texels the shore takes a side (item 162).
// Ebitengine rounds a texture up to a power of two, so the real history's
// 1464×1416 at 4 world units a texel was 2048², 16 MiB; at 1024 it is
// 4 MiB, and the bands are soft gradients drawn with linear filtering.
const shoreTexelSide = 1024

// shoreTexelFor is the texel for an island's sea: 4 world units, or as
// coarse as fits the longer side in shoreTexelSide texels.
func shoreTexelFor(sea city.Rect) float64 {
	return max(shoreTexel, max(sea.Width(), sea.Height())/shoreTexelSide)
}

// shoreSize is the shore texture's size for an island's sea.
func shoreSize(sea city.Rect) (w, h int) {
	texel := shoreTexelFor(sea)
	return int(sea.Width() / texel), int(sea.Height() / texel)
}

// shoreGeoM maps the shore texture's texels onto the screen.
func shoreGeoM(cam *city.Camera, sea city.Rect) ebiten.GeoM {
	texel := shoreTexelFor(sea)
	project := func(p city.Point) city.Point { return cam.Project(p).Add(cam.Offset).Scale(cam.Zoom) }
	o := project(sea.Min)
	ex := project(city.Point{X: sea.Min.X + texel, Y: sea.Min.Y}).Sub(o)
	ey := project(city.Point{X: sea.Min.X, Y: sea.Min.Y + texel}).Sub(o)
	var m ebiten.GeoM
	m.SetElement(0, 0, ex.X)
	m.SetElement(1, 0, ex.Y)
	m.SetElement(0, 1, ey.X)
	m.SetElement(1, 1, ey.Y)
	m.SetElement(0, 2, o.X)
	m.SetElement(1, 2, o.Y)
	return m
}

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

// shorePaletteFor is the island's palette by day or by night.
func shorePaletteFor(night bool) shorePalette {
	if night {
		return nightShore
	}
	return dayShore
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
	sea    city.Rect
	img    *ebiten.Image
}

func (g *Game) shoreImage(c *city.City) *shoreLayer {
	island := c.Island()
	if l := g.shoreLayer; l != nil && l.island == island && l.night == c.Night {
		return l
	}
	palette := shorePaletteFor(c.Night)
	sea := island.Sea()
	w, h := shoreSize(sea)
	texel := shoreTexelFor(sea)
	pix := make([]byte, 4*w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			kind, t := island.At(city.Point{X: sea.Min.X + (float64(x)+0.5)*texel, Y: sea.Min.Y + (float64(y)+0.5)*texel})
			col := palette.paint(kind, t)
			i := 4 * (y*w + x)
			pix[i], pix[i+1], pix[i+2], pix[i+3] = col.R, col.G, col.B, 0xff
		}
	}
	img := ebiten.NewImageWithOptions(image.Rect(0, 0, w, h), shoreOptions())
	img.WritePixels(pix)
	if g.shoreLayer != nil {
		g.shoreLayer.img.Deallocate()
	}
	g.shoreLayer = &shoreLayer{island: island, night: c.Night, sea: sea, img: img}
	return g.shoreLayer
}

// shoreOptions makes the shore its own texture (item 162). On
// Ebitengine's shared atlas the real history's shore, 1464×1416, grew the
// atlas to 4096×2048, which never shrinks: 32 MiB for 9.4 MiB of pixels.
func shoreOptions() *ebiten.NewImageOptions {
	return &ebiten.NewImageOptions{Unmanaged: true}
}

// shore draws the island's ground and the sea under the plan.
func (g *Game) shore(screen *ebiten.Image, cam *city.Camera, c *city.City) {
	if c.Bounds().Area() == 0 {
		return
	}
	s := g.shoreImage(c)
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear, GeoM: shoreGeoM(cam, s.sea)}
	g.recorder().Image(screen, s.img, op)
}
