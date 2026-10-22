package ui_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 159: any wait before the city is drawn shows a bar with real
// progress instead of an empty window, but not a wait too short to read.
func TestOpeningShown(t *testing.T) {
	for _, tc := range []struct {
		waited time.Duration
		shown  bool
	}{
		{0, false},
		{299 * time.Millisecond, false},
		{300 * time.Millisecond, true},
		{4 * time.Second, true},
	} {
		t.Run(fmt.Sprintf("when the city has waited %v", tc.waited), func(t *testing.T) {
			t.Run(fmt.Sprintf("it should show the panel: %v", tc.shown), func(t *testing.T) {
				assert.Equal(t, tc.shown, ui.OpeningShown(tc.waited))
			})
		})
	}
}

func TestOpeningLine(t *testing.T) {
	t.Run("when the pages are going up", func(t *testing.T) {
		line, fraction := ui.OpeningPages(1, 2)

		t.Run("it should say the city is opening", func(t *testing.T) {
			assert.Equal(t, "Opening the city", line)
		})

		t.Run("it should fill to the pages up", func(t *testing.T) {
			assert.InDelta(t, 0.5, fraction, 1e-9)
		})
	})

	for _, total := range []int{2, 6} {
		t.Run(fmt.Sprintf("when all %d pages are up", total), func(t *testing.T) {
			_, fraction := ui.OpeningPages(total, total)

			t.Run("it should be full", func(t *testing.T) {
				assert.InDelta(t, 1.0, fraction, 1e-9)
			})
		})
	}

	t.Run("when the daemon has read 40 of 120 sessions", func(t *testing.T) {
		line, fraction := ui.OpeningLoad(state.LoadProgress{Done: 40, Total: 120})

		t.Run("it should say so", func(t *testing.T) {
			assert.Equal(t, "Reading your sessions · 40 of 120", line)
		})

		t.Run("it should fill to a third", func(t *testing.T) {
			assert.InDelta(t, 1.0/3, fraction, 1e-9)
		})
	})

	t.Run("when nothing has said how far the sessions have been read", func(t *testing.T) {
		line, fraction := ui.OpeningLoad(state.LoadProgress{})

		t.Run("it should say they are being read", func(t *testing.T) {
			assert.Equal(t, "Reading your sessions", line)
		})

		t.Run("it should claim no fill", func(t *testing.T) {
			assert.Equal(t, ui.NoFill, fraction)
		})
	})
}

func TestLayoutOpening(t *testing.T) {
	th := ui.NewTheme(1)

	t.Run("when the panel is laid out in a 1600×900 window, a third full", func(t *testing.T) {
		o := ui.LayoutOpening(th, 1600, 900, "Opening the city", 1.0/3, measure7)

		t.Run("it should centre the panel", func(t *testing.T) {
			assert.InDelta(t, 800, (o.Panel.Min.X+o.Panel.Max.X)/2, 0.5)
		})

		t.Run("it should hold the line inside the panel", func(t *testing.T) {
			assert.True(t, o.Panel.Contains(o.Line.At))
		})

		t.Run("it should hold the bar inside the panel", func(t *testing.T) {
			assert.True(t, o.Panel.Contains(o.Bar.Min) && o.Panel.Contains(o.Bar.Max))
		})

		t.Run("it should start the fill where the bar starts", func(t *testing.T) {
			assert.Equal(t, o.Bar.Min, o.Fill.Min)
		})

		t.Run("it should fill a third of the bar", func(t *testing.T) {
			assert.InDelta(t, o.Bar.Width()/3, o.Fill.Width(), 1e-6)
		})
	})

	t.Run("when there is no fill to show", func(t *testing.T) {
		at := func(phase float64) ui.Opening {
			return ui.LayoutOpening(th, 1600, 900, "Reading your sessions", ui.NoFill, measure7).Sweep(phase)
		}
		early, late := at(0.1), at(0.6)

		t.Run("it should sweep a fifth of the bar across it", func(t *testing.T) {
			assert.InDelta(t, early.Bar.Width()/5, early.Fill.Width(), 1e-6)
		})

		t.Run("it should move as time passes", func(t *testing.T) {
			assert.Less(t, early.Fill.Min.X, late.Fill.Min.X)
		})

		t.Run("it should stay inside the bar", func(t *testing.T) {
			assert.LessOrEqual(t, late.Fill.Max.X, late.Bar.Max.X)
		})
	})

	t.Run("when there is a fill", func(t *testing.T) {
		o := ui.LayoutOpening(th, 1600, 900, "Opening the city", 0.5, measure7)

		t.Run("it should not sweep", func(t *testing.T) {
			assert.Equal(t, o, o.Sweep(0.6))
		})
	})
}
