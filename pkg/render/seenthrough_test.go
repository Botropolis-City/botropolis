package render

import (
	"fmt"
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Item 162, slice 14: z1.5 drawn up to zoom 2 widens a building's soft
// rim, and the occlusion checks read the rim as a window showing through.
// The margin they take as solid follows the draw-up, and this holds that
// a light over the building's interior is still caught at every margin.
func TestSolidMargin(t *testing.T) {
	for _, tc := range []struct {
		drawnUp float64
		margin  int
	}{
		{1, 2}, {4.0 / 3, 4}, {1.5, 4}, {1.98, 5},
	} {
		t.Run(fmt.Sprintf("when a sprite is drawn up %.2fx", tc.drawnUp), func(t *testing.T) {
			t.Run(fmt.Sprintf("it should take pixels %d in from its edge as solid", tc.margin), func(t *testing.T) {
				assert.Equal(t, tc.margin, solidMarginFor(tc.drawnUp))
			})
		})
	}
}

func TestSeenThroughFront(t *testing.T) {
	const side, from, to = 64, 12, 52
	dark := color.NRGBA{0x0e, 0x10, 0x1f, 0xff}
	wall := color.NRGBA{0x66, 0x6c, 0x80, 0xff}
	fill := func(c color.NRGBA) *image.NRGBA {
		img := image.NewNRGBA(image.Rect(0, 0, side, side))
		for y := range side {
			for x := range side {
				img.SetNRGBA(x, y, c)
			}
		}
		return img
	}
	blank := fill(dark)
	mask := fill(dark)
	for y := from; y < to; y++ {
		for x := from; x < to; x++ {
			mask.SetNRGBA(x, y, wall)
		}
	}
	litAt := func(x, y int) (on, off *image.NRGBA) {
		on, off = fill(wall), fill(wall)
		for dy := range 2 {
			for dx := range 2 {
				on.SetNRGBA(x+dx, y+dy, color.NRGBA{0xf6, 0xe0, 0x8a, 0xff})
			}
		}
		return on, off
	}

	for _, drawnUp := range []float64{1, 4.0 / 3, 1.98} {
		margin := solidMarginFor(drawnUp)
		t.Run(fmt.Sprintf("when the building in front is drawn up %.2fx", drawnUp), func(t *testing.T) {
			t.Run("and a light falls on its interior", func(t *testing.T) {
				on, off := litAt(side/2, side/2)

				t.Run("it should report it", func(t *testing.T) {
					assert.NotEmpty(t, seenThroughFront("interior", on, off, mask, blank, margin))
				})
			})

			t.Run("and a light falls on its rim", func(t *testing.T) {
				on, off := litAt(from+margin-2, side/2)

				t.Run("it should not report it", func(t *testing.T) {
					assert.Empty(t, seenThroughFront("rim", on, off, mask, blank, margin))
				})
			})
		})
	}
}
