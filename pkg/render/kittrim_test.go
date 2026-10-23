package render

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/assets"
)

// Item 162: every page was a whole 2048-pixel texture however little of
// it the packer filled; z2's last page uses 701 of its rows and z1's 943.
func TestKitPageTrim(t *testing.T) {
	k, err := loadKits()
	require.NoError(t, err)
	type page struct {
		bounds  image.Rectangle
		lowest  int
		sprites []image.Rectangle
	}
	var pages []page
	for _, a := range k.atlases {
		these := make([]page, len(a.pages))
		for i, p := range a.pages {
			these[i].bounds = p.Bounds()
		}
		for _, name := range a.Names() {
			for _, heading := range assets.Headings {
				s, _ := a.Sprite(name, heading)
				these[s.Page].lowest = max(these[s.Page].lowest, s.Rect.Max.Y)
				these[s.Page].sprites = append(these[s.Page].sprites, s.Rect)
			}
		}
		pages = append(pages, these...)
	}
	require.NotEmpty(t, pages)

	t.Run("when every page of both levels is put up", func(t *testing.T) {
		t.Run("it should put up no texture taller than its lowest sprite's bottom row", func(t *testing.T) {
			var over []int
			for _, p := range pages {
				if p.bounds.Dy() > p.lowest {
					over = append(over, p.bounds.Dy()-p.lowest)
				}
			}
			assert.Empty(t, over)
		})

		t.Run("it should keep every sprite inside its page's texture", func(t *testing.T) {
			var outside []image.Rectangle
			for _, p := range pages {
				for _, r := range p.sprites {
					if !r.In(p.bounds) {
						outside = append(outside, r)
					}
				}
			}
			assert.Empty(t, outside)
		})
	})
}

// Item 162: Ebitengine rounds every texture up to a power of two a side,
// so what a level costs is its pages at those sizes. Packed in shelves in
// the order they were cut, z2 was five full pages and a part, 88 MiB;
// filled a page at a time it is four and a 2048×1024 one, 72.
func TestKitTextureCost(t *testing.T) {
	k, err := loadKits()
	require.NoError(t, err)
	cost := func(level int) int {
		total := 0
		for _, p := range k.atlases[level].pages {
			b := p.Bounds()
			total += 4 * internalSide(b.Dx()) * internalSide(b.Dy())
		}
		return total >> 20
	}

	t.Run("when every page of both levels is put up", func(t *testing.T) {
		t.Run("it should hold z1 in no more than 20 MiB of texture", func(t *testing.T) {
			assert.LessOrEqual(t, cost(0), 20)
		})

		t.Run("it should hold z2 in no more than 72 MiB of texture", func(t *testing.T) {
			assert.LessOrEqual(t, cost(1), 72)
		})
	})
}

// internalSide is a texture side as Ebitengine allocates it
// (internal/graphics.InternalImageSize, v2.10).
func internalSide(n int) int {
	side := 16
	for side < n {
		side <<= 1
	}
	return side
}
