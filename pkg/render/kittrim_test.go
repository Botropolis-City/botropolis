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
