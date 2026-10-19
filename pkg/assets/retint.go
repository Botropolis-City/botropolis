package assets

import (
	"image"
	"image/color"
	"image/draw"
	"math"
)

// The kits paint one amber accent on the plant's stack and the towers'
// tanks, and amber is the map's needs-you colour; so those bands are
// retinted to the plant's own tone when the atlas loads, and the one
// colour keeps its one meaning. Decided 2026-09-18.

// Amber is the kit accent's hue range, in degrees, and the saturation
// below which a pixel is not the accent but a shadow of it.
const (
	amberLow       = 18.0
	amberHigh      = 58.0
	amberSaturated = 0.4
)

// RetintAmber turns every amber pixel of the named sprites, at every
// heading, to the target's hue, keeping each pixel's saturation and
// lightness so the shading survives.
// A page decoded after this call is retinted as it is decoded.
func (a *KitAtlas) RetintAmber(names []string, target color.NRGBA) {
	r := retint{names: names, target: target}
	a.retints = append(a.retints, r)
	for i := range a.Pages {
		a.retintPage(i, a.mutablePage(i), r)
	}
}

type retint struct {
	names  []string
	target color.NRGBA
}

func (a *KitAtlas) retintPage(i int, page *image.NRGBA, r retint) {
	hue, _, _ := hsl(r.target)
	for _, name := range r.names {
		set, ok := a.sprites[name]
		if !ok {
			continue
		}
		for _, s := range set {
			if s.Page != i {
				continue
			}
			for y := s.Rect.Min.Y; y < s.Rect.Max.Y; y++ {
				for x := s.Rect.Min.X; x < s.Rect.Max.X; x++ {
					c := page.NRGBAAt(x, y)
					h, sat, l := hsl(c)
					if c.A == 0 || sat < amberSaturated || h < amberLow || h > amberHigh {
						continue
					}
					page.SetNRGBA(x, y, fromHSL(hue, sat, l, c.A))
				}
			}
		}
	}
}

func (a *KitAtlas) mutablePage(i int) *image.NRGBA {
	p := toNRGBA(a.Pages[i])
	a.Pages[i] = p
	return p
}

func toNRGBA(img image.Image) *image.NRGBA {
	if p, ok := img.(*image.NRGBA); ok {
		return p
	}
	b := img.Bounds()
	p := image.NewNRGBA(b)
	draw.Draw(p, b, img, b.Min, draw.Src)
	return p
}

// hsl is a colour's hue in degrees, saturation and lightness in 0..1.
func hsl(c color.NRGBA) (float64, float64, float64) {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	hi, lo := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	l := (hi + lo) / 2
	if hi == lo {
		return 0, 0, l
	}
	d := hi - lo
	s := d / (1 - math.Abs(2*l-1))
	var h float64
	switch hi {
	case r:
		h = math.Mod((g-b)/d, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return h, s, l
}

func fromHSL(h, s, l float64, alpha uint8) color.NRGBA {
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	to := func(v float64) uint8 { return uint8(math.Round(math.Min(1, math.Max(0, v+m)) * 255)) }
	return color.NRGBA{R: to(r), G: to(g), B: to(b), A: alpha}
}
