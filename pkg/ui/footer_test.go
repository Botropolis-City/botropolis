package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// The footer is the key row: a passing message is a notice under the
// strip, never a line down here (item 352).
func TestLayoutFooter(t *testing.T) {
	th := ui.NewTheme(1)
	keys := []ui.Key{{"drag", "pan"}, {"tab", "next needs-you"}}
	f := ui.LayoutFooter(th, 800, 600, keys, measure7)

	t.Run("it should span the width along the bottom", func(t *testing.T) {
		assert.Equal(t, city.RectAt(0, 600-f.Rect.Height(), 800, f.Rect.Height()), f.Rect)
	})

	t.Run("it should be a line and two grid units tall", func(t *testing.T) {
		assert.Equal(t, 32.0, f.Rect.Height())
	})

	t.Run("it should make the key row the whole footer", func(t *testing.T) {
		assert.Equal(t, f.Rect, f.KeyRow)
	})

	t.Run("it should lay out every key", func(t *testing.T) {
		assert.Len(t, f.Keys, 2)
	})

	t.Run("it should start the first key chip two grid units in", func(t *testing.T) {
		assert.Equal(t, 16.0, f.Keys[0].Cap.Rect.Min.X)
	})

	t.Run("it should pad the key chip half a grid each side", func(t *testing.T) {
		assert.Equal(t, 4*7+8.0, f.Keys[0].Cap.Rect.Width())
	})

	t.Run("it should put the label one grid unit after the chip", func(t *testing.T) {
		assert.Equal(t, f.Keys[0].Cap.Rect.Max.X+8, f.Keys[0].LabelAt.X)
	})

	t.Run("it should start the next key three grid units after the label", func(t *testing.T) {
		assert.Equal(t, f.Keys[0].LabelAt.X+3*7+24, f.Keys[1].Cap.Rect.Min.X)
	})

	t.Run("it should draw each key as a key cap", func(t *testing.T) {
		assert.Equal(t, ui.LayoutKeyCap(th, "drag", f.Keys[0].Cap.Rect.Min, measure7), f.Keys[0].Cap)
	})

	t.Run("it should drop keys that do not fit", func(t *testing.T) {
		narrow := ui.LayoutFooter(th, 120, 600, keys, measure7)
		assert.Len(t, narrow.Keys, 1)
	})
}
