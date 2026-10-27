package tray

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 293: the tray's icon is the logo's twelve polygons
// (packaging/botropolis.svg), drawn in code at the sizes trays ask for.
func TestIcon(t *testing.T) {
	for _, size := range iconSizes {
		t.Run(fmt.Sprintf("when drawn at %d px", size), func(t *testing.T) {
			img := icon(size)

			t.Run("it should be that size square", func(t *testing.T) {
				assert.Equal(t, [2]int{size, size}, [2]int{img.Bounds().Dx(), img.Bounds().Dy()})
			})

			// At 16 px the corner's radius is 3 px, which still grazes
			// the corner pixel.
			t.Run("it should leave the rounded corner nearly clear", func(t *testing.T) {
				assert.Less(t, img.RGBAAt(0, 0).A, uint8(0x10))
			})

			t.Run("it should fill the middle with the plate", func(t *testing.T) {
				assert.Equal(t, uint8(0xff), img.RGBAAt(size/2, size/2).A)
			})
		})
	}

	t.Run("when converted to a StatusNotifierItem pixmap", func(t *testing.T) {
		img := icon(iconSizes[0])
		px := pixmap(img)
		c := img.RGBAAt(img.Bounds().Dx()/2, img.Bounds().Dy()/2)
		i := 4 * (img.Bounds().Dy()/2*img.Bounds().Dx() + img.Bounds().Dx()/2)
		require.Len(t, px.Pixels, 4*img.Bounds().Dx()*img.Bounds().Dy())

		t.Run("it should put each pixel as alpha, red, green, blue", func(t *testing.T) {
			assert.Equal(t, []byte{c.A, c.R, c.G, c.B}, px.Pixels[i:i+4])
		})
	})
}
