package ui_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

func TestLayoutFooter(t *testing.T) {
	th := ui.NewTheme(1)
	keys := []ui.Key{{"drag", "pan"}, {"tab", "next needs-you"}}

	t.Run("when a status is given", func(t *testing.T) {
		f := ui.LayoutFooter(th, 800, 600, "attached: Fix the CI queue", keys, measure7)

		t.Run("it should span the width along the bottom", func(t *testing.T) {
			assert.Equal(t, city.RectAt(0, 600-f.Rect.Height(), 800, f.Rect.Height()), f.Rect)
		})

		t.Run("it should be the key row plus a line and a grid tall", func(t *testing.T) {
			assert.Equal(t, 32+16+8.0, f.Rect.Height())
		})

		t.Run("it should show the status", func(t *testing.T) {
			require.Len(t, f.Lines, 1)
			assert.Equal(t, "attached: Fix the CI queue", f.Lines[0].Text)
		})

		t.Run("it should start the text two grid units in", func(t *testing.T) {
			assert.Equal(t, city.Point{X: 16, Y: 600 - 56 + 8}, f.Lines[0].At)
		})

		t.Run("it should keep the key row", func(t *testing.T) {
			assert.Len(t, f.Keys, 2)
		})

		t.Run("it should put the status above the keys", func(t *testing.T) {
			assert.Less(t, f.Lines[0].At.Y, f.Keys[0].KeyAt.Y)
		})

		t.Run("it should keep the key row's own height apart, for the map's reserve", func(t *testing.T) {
			assert.Equal(t, 32.0, f.KeyRow.Height())
		})
	})

	t.Run("when the status is longer than the width", func(t *testing.T) {
		long := strings.Repeat("word ", 60)
		f := ui.LayoutFooter(th, 300, 600, long, keys, measure7)

		t.Run("it should wrap it", func(t *testing.T) {
			assert.Greater(t, len(f.Lines), 1)
		})

		t.Run("it should cap at four lines", func(t *testing.T) {
			assert.LessOrEqual(t, len(f.Lines), 4)
		})

		t.Run("it should grow with the lines above the key row", func(t *testing.T) {
			assert.Equal(t, 32+16*float64(len(f.Lines))+8, f.Rect.Height())
		})
	})

	t.Run("when no status is given", func(t *testing.T) {
		f := ui.LayoutFooter(th, 800, 600, "", keys, measure7)

		t.Run("it should show no lines", func(t *testing.T) {
			assert.Empty(t, f.Lines)
		})

		t.Run("it should make the key row the whole footer", func(t *testing.T) {
			assert.Equal(t, f.Rect, f.KeyRow)
		})

		t.Run("it should lay out every key", func(t *testing.T) {
			assert.Len(t, f.Keys, 2)
		})

		t.Run("it should start the first key chip two grid units in", func(t *testing.T) {
			assert.Equal(t, 16.0, f.Keys[0].Chip.Min.X)
		})

		t.Run("it should pad the key chip half a grid each side", func(t *testing.T) {
			assert.Equal(t, 4*7+8.0, f.Keys[0].Chip.Width())
		})

		t.Run("it should put the label one grid unit after the chip", func(t *testing.T) {
			assert.Equal(t, f.Keys[0].Chip.Max.X+8, f.Keys[0].LabelAt.X)
		})

		t.Run("it should start the next key three grid units after the label", func(t *testing.T) {
			assert.Equal(t, f.Keys[0].LabelAt.X+3*7+24, f.Keys[1].Chip.Min.X)
		})

		t.Run("it should drop keys that do not fit", func(t *testing.T) {
			narrow := ui.LayoutFooter(th, 120, 600, "", keys, measure7)
			assert.Len(t, narrow.Keys, 1)
		})
	})
}
