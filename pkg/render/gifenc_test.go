package render

import (
	"image"
	"image/color"
	stddraw "image/draw"
	"image/gif"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 188: one palette for the whole GIF, and each frame after the
// first only what changed, so a still region never shimmers.

// encodeGIF writes frames through the GIF writer and reads the file back.
func encodeGIF(t *testing.T, frames []*image.RGBA) *gif.GIF {
	t.Helper()
	path := filepath.Join(t.TempDir(), "clip.gif")
	w := newGIFWriter(path, 30)
	for _, f := range frames {
		w.add(f)
	}
	require.NoError(t, <-w.finish())
	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	anim, err := gif.DecodeAll(f)
	require.NoError(t, err)
	return anim
}

// composite is what a viewer shows at each frame: every frame drawn over
// the one before, as a GIF with frames left in place is played.
func composite(anim *gif.GIF) []*image.RGBA {
	canvas := image.NewRGBA(image.Rect(0, 0, anim.Config.Width, anim.Config.Height))
	out := make([]*image.RGBA, 0, len(anim.Image))
	for _, p := range anim.Image {
		stddraw.Draw(canvas, p.Bounds(), p, p.Bounds().Min, stddraw.Over)
		shown := image.NewRGBA(canvas.Bounds())
		copy(shown.Pix, canvas.Pix)
		out = append(out, shown)
	}
	return out
}

func gradient(img *image.RGBA, r image.Rectangle, seed int) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8(x*2 + seed*37), uint8(y*4 + seed*11), uint8(255 - x - seed*53), 0xff})
		}
	}
}

func TestGIFWriter(t *testing.T) {
	t.Run("when frames share a still region with thousands of colours, and the rest changes every frame", func(t *testing.T) {
		const w, h = 256, 64
		still := image.Rect(0, 0, w/2, h)
		moving := image.Rect(w/2, 0, w, h)
		var frames []*image.RGBA
		for i := range 6 {
			img := image.NewRGBA(image.Rect(0, 0, w, h))
			gradient(img, still, 0)
			gradient(img, moving, i+1)
			frames = append(frames, img)
		}
		anim := encodeGIF(t, frames)
		shown := composite(anim)

		t.Run("it should draw the still region identically in every frame", func(t *testing.T) {
			var differ int
			for _, s := range shown[1:] {
				for y := still.Min.Y; y < still.Max.Y; y++ {
					for x := still.Min.X; x < still.Max.X; x++ {
						if s.RGBAAt(x, y) != shown[0].RGBAAt(x, y) {
							differ++
						}
					}
				}
			}
			assert.Zero(t, differ)
		})

		t.Run("it should use one palette for every frame", func(t *testing.T) {
			var other int
			for _, p := range anim.Image[1:] {
				if !slicesEqualPalette(p.Palette, anim.Image[0].Palette) {
					other++
				}
			}
			assert.Zero(t, other)
		})

		t.Run("it should encode each later frame no bigger than the box that changed", func(t *testing.T) {
			var outside []image.Rectangle
			for _, p := range anim.Image[1:] {
				if !p.Bounds().In(moving) {
					outside = append(outside, p.Bounds())
				}
			}
			assert.Empty(t, outside)
		})
	})

	t.Run("when frames have fewer than 256 colours between them", func(t *testing.T) {
		colours := []color.RGBA{{0x10, 0x20, 0x30, 0xff}, {0xf0, 0xb4, 0x4c, 0xff}, {0x5c, 0xc8, 0xb0, 0xff}}
		var frames []*image.RGBA
		for i := range 3 {
			img := image.NewRGBA(image.Rect(0, 0, 40, 30))
			for y := range 30 {
				for x := range 40 {
					img.SetRGBA(x, y, colours[(x+y+i)%len(colours)])
				}
			}
			frames = append(frames, img)
		}
		shown := composite(encodeGIF(t, frames))

		t.Run("it should keep every pixel's colour", func(t *testing.T) {
			var changed int
			for i, s := range shown {
				for j := 0; j < len(s.Pix); j++ {
					if s.Pix[j] != frames[i].Pix[j] {
						changed++
					}
				}
			}
			assert.Zero(t, changed)
		})
	})

	t.Run("when frames share thousands of colours, each drawn somewhere else in each frame", func(t *testing.T) {
		var frames []*image.RGBA
		for i := range 3 {
			src := image.NewRGBA(image.Rect(0, 0, 256, 64))
			gradient(src, src.Bounds(), 0)
			img := image.NewRGBA(src.Bounds())
			for y := range 64 {
				for x := range 256 {
					img.SetRGBA((x+i*85)%256, (y+i*21)%64, src.RGBAAt(x, y))
				}
			}
			frames = append(frames, img)
		}
		shown := composite(encodeGIF(t, frames))
		var worst int
		for i, s := range shown {
			for j := 0; j < len(s.Pix); j += 4 {
				for k := range 3 {
					worst = max(worst, absDiff(s.Pix[j+k], frames[i].Pix[j+k]))
				}
			}
		}

		t.Run("it should keep every channel within 16 of 255 of its colour (gifShade)", func(t *testing.T) {
			assert.LessOrEqual(t, worst, gifShade)
		})
	})
}

func slicesEqualPalette(a, b color.Palette) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if color.RGBAModel.Convert(a[i]) != color.RGBAModel.Convert(b[i]) {
			return false
		}
	}
	return true
}
