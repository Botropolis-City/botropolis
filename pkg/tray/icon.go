package tray

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"golang.org/x/image/vector"
)

// iconSizes are the sizes trays ask for, smallest first.
var iconSizes = []int{16, 22, 24, 32, 48, 64}

// sniPixmap is one size of a StatusNotifierItem's IconPixmap.
type sniPixmap struct {
	Width, Height int32
	Pixels        []byte
}

type point struct{ x, y float32 }

type shape struct {
	fill    color.RGBA
	polygon []point
	circle  *struct{ x, y, r float32 }
}

func rgb(hex uint32) color.RGBA {
	return color.RGBA{uint8(hex >> 16), uint8(hex >> 8), uint8(hex), 0xff}
}

func circle(x, y, r float32) *struct{ x, y, r float32 } {
	return &struct{ x, y, r float32 }{x, y, r}
}

// logo is packaging/botropolis.svg, on its 64-unit grid: the plate, the
// ground's three faces, and two buildings of three faces and a window
// each. Geometry placed by hand, as design/art-sources.md asks.
var logo = []shape{
	{fill: rgb(0x829c6e), polygon: []point{{32, 10}, {58, 23}, {32, 36}, {6, 23}}},
	{fill: rgb(0x3c4a36), polygon: []point{{32, 36}, {58, 23}, {58, 31}, {32, 44}}},
	{fill: rgb(0x526644), polygon: []point{{6, 23}, {32, 36}, {32, 44}, {6, 31}}},
	{fill: rgb(0xe6e9ef), polygon: []point{{22, 18}, {30, 22}, {30, 34}, {22, 30}}},
	{fill: rgb(0xb3b7c2), polygon: []point{{30, 22}, {38, 18}, {38, 30}, {30, 34}}},
	{fill: rgb(0x5c5e66), polygon: []point{{22, 18}, {30, 14}, {38, 18}, {30, 22}}},
	{fill: rgb(0xe8a03c), circle: circle(26, 27, 2.2)},
	{fill: rgb(0xe6e9ef), polygon: []point{{40, 23}, {46, 26}, {46, 34}, {40, 31}}},
	{fill: rgb(0xb3b7c2), polygon: []point{{46, 26}, {52, 23}, {52, 31}, {46, 34}}},
	{fill: rgb(0x5c5e66), polygon: []point{{40, 23}, {46, 20}, {52, 23}, {46, 26}}},
	{fill: rgb(0x559ae0), circle: circle(43, 29, 2.2)},
}

const (
	logoGrid   = 64
	plateR     = 12
	plateColor = 0x12141a
)

// icon draws the logo at size pixels square.
func icon(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	k := float32(size) / logoGrid
	fill := func(c color.RGBA, path func(r *vector.Rasterizer)) {
		r := vector.NewRasterizer(size, size)
		r.DrawOp = draw.Over
		path(r)
		r.ClosePath()
		r.Draw(img, img.Bounds(), image.NewUniform(c), image.Point{})
	}
	fill(rgb(plateColor), func(r *vector.Rasterizer) { roundRect(r, logoGrid*k, plateR*k) })
	for _, s := range logo {
		if s.circle != nil {
			fill(s.fill, func(r *vector.Rasterizer) { disc(r, s.circle.x*k, s.circle.y*k, s.circle.r*k) })
			continue
		}
		fill(s.fill, func(r *vector.Rasterizer) {
			r.MoveTo(s.polygon[0].x*k, s.polygon[0].y*k)
			for _, p := range s.polygon[1:] {
				r.LineTo(p.x*k, p.y*k)
			}
		})
	}
	return img
}

// roundRect is the plate: a square with corners of a quarter circle,
// each a cubic whose handles stand kappa of the radius from its ends.
func roundRect(r *vector.Rasterizer, side, radius float32) {
	const kappa = 0.5523
	h := radius * (1 - kappa)
	r.MoveTo(radius, 0)
	r.LineTo(side-radius, 0)
	r.CubeTo(side-h, 0, side, h, side, radius)
	r.LineTo(side, side-radius)
	r.CubeTo(side, side-h, side-h, side, side-radius, side)
	r.LineTo(radius, side)
	r.CubeTo(h, side, 0, side-h, 0, side-radius)
	r.LineTo(0, radius)
	r.CubeTo(0, h, h, 0, radius, 0)
}

func disc(r *vector.Rasterizer, cx, cy, radius float32) {
	const steps = 24
	r.MoveTo(cx+radius, cy)
	for i := 1; i < steps; i++ {
		a := 2 * math.Pi * float64(i) / steps
		r.LineTo(cx+radius*float32(math.Cos(a)), cy+radius*float32(math.Sin(a)))
	}
}

// pixmap is img as StatusNotifierItem wants it: ARGB32, in network byte
// order, not premultiplied.
func pixmap(img *image.RGBA) sniPixmap {
	b := img.Bounds()
	px := make([]byte, 0, 4*b.Dx()*b.Dy())
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.RGBAAt(x, y)).(color.NRGBA)
			px = append(px, c.A, c.R, c.G, c.B)
		}
	}
	return sniPixmap{Width: int32(b.Dx()), Height: int32(b.Dy()), Pixels: px}
}
