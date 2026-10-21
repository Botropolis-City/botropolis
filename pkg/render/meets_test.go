package render

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Item 193: a cut is laid before a window it meets, so it lands in the
// painters' order whatever other windows are laid; a pane's light
// spreads litMargin past the pane, and a cut over only that spread
// meets it too, or where it lands would hang on some other window.
func TestMeets(t *testing.T) {
	pane := []image.Rectangle{image.Rect(10, 10, 14, 20)}
	for _, c := range []struct {
		name string
		cut  image.Rectangle
		want bool
	}{
		{"when a cut covers the pane", image.Rect(11, 11, 13, 13), true},
		{"when a cut covers only the pixel above the pane its light spreads into", image.Rect(10, 9, 14, 10), true},
		{"when a cut covers only the pixel beside the pane", image.Rect(14, 12, 15, 13), true},
		{"when a cut is two pixels clear of the pane", image.Rect(16, 12, 18, 13), false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Run("it should say whether they meet", func(t *testing.T) {
				assert.Equal(t, c.want, meets(c.cut, pane))
			})
		})
	}
}
