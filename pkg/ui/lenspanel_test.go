package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

func TestLayoutLensPanel(t *testing.T) {
	th := ui.NewTheme(1)
	bounds := city.RectAt(0, 32, 1200, 760)
	panel := city.LensPanel{
		Lens:      city.LensCapacity,
		Name:      "Capacity",
		About:     "How full each session's context window is; the red ones will compact soon.",
		Legend:    city.Legend{Shown: true, Ramp: true, Low: "0", High: "100% of the window"},
		Aggregate: "3 sessions over 80%",
		Do:        "Click a red one to see when it last compacted.",
	}
	laid := ui.LayoutLensPanel(th, panel, bounds, measure7)

	t.Run("it should set the name at display size", func(t *testing.T) {
		assert.Equal(t, []any{"Capacity", ui.Display}, []any{laid.Name.Text, laid.Name.Size})
	})

	t.Run("it should put the legend's units at both ends", func(t *testing.T) {
		require.Greater(t, laid.Bar.Width(), 0.0)
		lowW, _ := measure7(laid.Low.Text, laid.Low.Size)
		assert.True(t, laid.Low.At.X+lowW <= laid.Bar.Min.X && laid.High.At.X >= laid.Bar.Max.X)
	})

	t.Run("it should shrink to a chip the width of the name", func(t *testing.T) {
		chipped := panel
		chipped.Chip = true
		c := ui.LayoutLensPanel(th, chipped, bounds, measure7)
		w, _ := measure7("Capacity", ui.Small)
		assert.Equal(t, []any{w + 2*th.Grid(), 0}, []any{c.Rect.Width(), len(c.About)})
	})
}
