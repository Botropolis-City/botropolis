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

// Item 347: the Connections lens carries a switch for where its lines run.
func TestLayoutLensPanelSwitch(t *testing.T) {
	th := ui.NewTheme(1)
	bounds := city.RectAt(0, 32, 1200, 760)
	panel := city.LensPanel{
		Lens:      city.LensConnections,
		Name:      "Connections",
		About:     "Which MCP servers each session calls.",
		Legend:    city.Legend{Shown: true, Categories: []string{"tracker", "weather"}},
		Aggregate: "3 lines from 2 towers to 2 sessions on the map",
		Do:        "Point at a tower to see who calls it.",
	}

	t.Run("when the panel carries a switch", func(t *testing.T) {
		withSwitch := panel
		withSwitch.Switch = "show lines to the depot too"
		laid := ui.LayoutLensPanel(th, withSwitch, bounds, measure7)

		t.Run("it should lay it under the aggregate", func(t *testing.T) {
			assert.Equal(t, withSwitch.Switch, laid.Switch.Text)
			assert.Greater(t, laid.Switch.At.Y, laid.Aggregate.At.Y)
		})

		t.Run("it should give it a rect to click, inside the panel", func(t *testing.T) {
			require.Greater(t, laid.SwitchRect.Area(), 0.0)
			assert.True(t, laid.SwitchRect.Contains(laid.Switch.At) && laid.Rect.Contains(laid.SwitchRect.Max))
		})
	})

	t.Run("when it carries none", func(t *testing.T) {
		laid := ui.LayoutLensPanel(th, panel, bounds, measure7)

		t.Run("the rect should be empty", func(t *testing.T) {
			assert.Equal(t, 0.0, laid.SwitchRect.Area())
		})
	})
}
