package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/image/font/sfnt"

	"botropolis.city/botropolis/pkg/assets/fonts"
	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 350: the menu buttons' mark showed as a box, since the font has
// no glyph for it, and so did every text field's caret. Every character
// the card's face, the reference and a field's caret draw must be one
// the font can draw.
func TestTheFontDrawsTheChrome(t *testing.T) {
	data, err := fonts.Inter()
	require.NoError(t, err)
	font, err := sfnt.Parse(data)
	require.NoError(t, err)
	var buf sfnt.Buffer
	missing := func(s string) []string {
		var out []string
		for _, r := range s {
			if r == ' ' || r == '\n' {
				continue
			}
			if i, _ := font.GlyphIndex(&buf, r); i == 0 {
				out = append(out, string(r))
			}
		}
		return out
	}
	texts := map[string]string{"the menu mark": city.MenuMark, "a field's caret": ui.Caret}
	for _, col := range city.VerbColumns {
		texts["the "+col.Title+" button"] = col.Title + " " + city.MenuMark
	}
	for _, k := range (&Game{}).helpKeys() {
		texts["the reference's "+k.Key+" row"] = k.Key + " " + k.Action
	}

	for name, text := range texts {
		t.Run("when "+name+" is drawn", func(t *testing.T) {
			t.Run("it should have a glyph for every character", func(t *testing.T) {
				assert.Empty(t, missing(text), text)
			})
		})
	}
}
