package render

import (
	"image"
	"image/color"
	imagedraw "image/draw"
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 162: Ebitengine copies every upload into a pooled buffer and keeps
// up to 31 released ones, so a page put up whole left 16 MiB on the heap
// for each of six pages, for the life of the city.
func TestKitPageStrips(t *testing.T) {
	type strip struct {
		pix    []byte
		region image.Rectangle
	}
	const width, height = 300, 2*stripRows + 17
	page := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			page.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: uint8(x ^ y), A: uint8(x * y)})
		}
	}
	var strips []strip
	pageStrips(page, func(pix []byte, region image.Rectangle) {
		strips = append(strips, strip{append([]byte(nil), pix...), region})
	})
	require.NotEmpty(t, strips)

	t.Run("when a page is cut into strips", func(t *testing.T) {
		t.Run("it should hand over no strip larger than the strip size", func(t *testing.T) {
			largest := 0
			for _, s := range strips {
				largest = max(largest, s.region.Dy())
			}
			assert.LessOrEqual(t, largest, stripRows)
		})

		t.Run("it should cover every row of the page exactly once", func(t *testing.T) {
			covered := make([]int, height)
			for _, s := range strips {
				for y := s.region.Min.Y; y < s.region.Max.Y; y++ {
					covered[y]++
				}
			}
			want := make([]int, height)
			for i := range want {
				want[i] = 1
			}
			assert.Equal(t, want, covered)
		})

		t.Run("it should hand over the same premultiplied pixels Ebitengine would make of the whole page", func(t *testing.T) {
			whole := image.NewRGBA(page.Rect)
			imagedraw.Draw(whole, whole.Rect, page, image.Point{}, imagedraw.Src)
			got := make([]byte, len(whole.Pix))
			for _, s := range strips {
				copy(got[s.region.Min.Y*whole.Stride:], s.pix)
			}
			assert.Equal(t, whole.Pix, got)
		})
	})

	// ebitenPooledBuffers, and sendPage's reason to exist, are read from
	// this version's source: a move has to read them again, or the 96 MB
	// can come back with every test green.
	t.Run("when the city is built", func(t *testing.T) {
		t.Run("it should use the Ebitengine those facts were read from", func(t *testing.T) {
			assert.Equal(t, kitsReadFromEbitengine, ebitengineVersion(t))
		})
	})

	t.Run("when the strip size is set for a full page", func(t *testing.T) {
		t.Run("it should keep what Ebitengine's pool can hold under 16 MiB", func(t *testing.T) {
			assert.LessOrEqual(t, ebitenPooledBuffers*4*kitPageSide*stripRows, 16<<20)
		})
	})
}

func ebitengineVersion(t *testing.T) string {
	t.Helper()
	info, ok := debug.ReadBuildInfo()
	require.True(t, ok)
	for _, d := range info.Deps {
		if d.Path == "github.com/hajimehoshi/ebiten/v2" {
			if d.Replace != nil {
				return d.Replace.Version
			}
			return d.Version
		}
	}
	t.Fatal("no Ebitengine in the build")
	return ""
}
