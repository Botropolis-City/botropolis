package tray

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Item 293: the XEmbed tray draws its own menu, so its face must hold
// every character of every label — Quit…'s ellipsis was drawn as a box
// in the first frame.
func TestMenuFace(t *testing.T) {
	face := menuFace()
	for _, e := range entries {
		t.Run("when "+e.label+" is drawn", func(t *testing.T) {
			var missing []string
			for _, r := range e.label {
				if _, ok := face.GlyphAdvance(r); !ok {
					missing = append(missing, string(r))
				}
			}

			t.Run("it should have a glyph for every character", func(t *testing.T) {
				assert.Empty(t, missing)
			})
		})
	}
}
