package assets_test

import (
	"fmt"
	"image"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/assets"
)

// Item 86: each window pane of a building, found in its rendered sprite,
// so the map can board or darken the windows a building actually has.
func TestKitPanes(t *testing.T) {
	atlases, err := assets.LoadKits()
	require.NoError(t, err)
	building := func(name string) bool {
		if strings.HasPrefix(name, "city-kit-commercial/building") || name == "city-kit-industrial/building-a" || name == "botropolis/city-hall" {
			return true
		}
		return regexp.MustCompile(`^botropolis/(residential|shopping|office|industrial)-[1-5]$`).MatchString(name)
	}

	for _, atlas := range atlases {
		for _, name := range atlas.Names() {
			for _, heading := range assets.Headings {
				sprite, _ := atlas.Sprite(name, heading)
				size := image.Rect(0, 0, sprite.Rect.Dx(), sprite.Rect.Dy())
				t.Run(fmt.Sprintf("when %s is seen from %d at zoom %g", name, heading, atlas.Zoom), func(t *testing.T) {
					t.Run("it should keep every pane inside its sprite", func(t *testing.T) {
						for _, pane := range sprite.Panes {
							for _, corner := range pane {
								assert.True(t, corner.In(size) || corner.X == size.Max.X || corner.Y == size.Max.Y, "%v outside %v", corner, size)
							}
						}
					})

					if !building(name) {
						t.Run("it should carry no panes, not being a building", func(t *testing.T) {
							assert.Empty(t, sprite.Panes)
						})
					}
				})
			}
			if building(name) {
				t.Run(fmt.Sprintf("when %s is cut at zoom %g", name, atlas.Zoom), func(t *testing.T) {
					panes := 0
					for _, heading := range assets.Headings {
						sprite, _ := atlas.Sprite(name, heading)
						panes += len(sprite.Panes)
					}

					t.Run("it should have panes from some heading", func(t *testing.T) {
						assert.Positive(t, panes)
					})
				})
			}
		}
	}

	t.Run("when the cottage is seen from its home heading in the closest level", func(t *testing.T) {
		closest := atlases[0]
		for _, atlas := range atlases {
			if atlas.Zoom > closest.Zoom {
				closest = atlas
			}
		}
		sprite, _ := closest.Sprite("botropolis/residential-1", 0)

		t.Run("it should have the five windows its two faces show", func(t *testing.T) {
			assert.Len(t, sprite.Panes, 5)
		})
	})
}
