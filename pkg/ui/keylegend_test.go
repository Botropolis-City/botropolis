package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 351: where a mode's keys show nowhere else, they are a legend: a
// cap and the word for what it does, pair after pair on one line.
func TestLayoutKeyLegend(t *testing.T) {
	th := ui.NewTheme(1)
	grid := th.Grid()
	at := city.Point{X: 40, Y: 300}
	keys := []ui.Key{{Key: "p", Action: "picture"}, {Key: "esc", Action: "leave"}}

	t.Run("when a legend is laid with room for every pair", func(t *testing.T) {
		l := ui.LayoutKeyLegend(th, keys, at, 600, measure7)
		require.Len(t, l.Keys, 2)

		t.Run("it should start its first cap where it is put", func(t *testing.T) {
			assert.Equal(t, at, l.Keys[0].Cap.Rect.Min)
		})

		t.Run("it should draw each key as a key cap", func(t *testing.T) {
			assert.Equal(t, ui.LayoutKeyCap(th, "esc", l.Keys[1].Cap.Rect.Min, measure7), l.Keys[1].Cap)
		})

		t.Run("it should set each word a grid after its cap, on the cap's line", func(t *testing.T) {
			assert.Equal(t, city.Point{X: l.Keys[0].Cap.Rect.Max.X + grid, Y: l.Keys[0].Cap.Label.At.Y}, l.Keys[0].LabelAt)
		})

		t.Run("it should start the next pair three grids after the last word", func(t *testing.T) {
			assert.Equal(t, l.Keys[0].LabelAt.X+7*7+3*grid, l.Keys[1].Cap.Rect.Min.X)
		})

		t.Run("it should span its pairs", func(t *testing.T) {
			assert.Equal(t, []float64{at.X, at.Y, l.Keys[1].LabelAt.X + 5*7}, []float64{l.Rect.Min.X, l.Rect.Min.Y, l.Rect.Max.X})
		})
	})

	t.Run("when the line is too narrow for every pair", func(t *testing.T) {
		l := ui.LayoutKeyLegend(th, keys, at, 100, measure7)

		t.Run("it should drop the pairs that do not fit", func(t *testing.T) {
			assert.Len(t, l.Keys, 1)
		})
	})
}
