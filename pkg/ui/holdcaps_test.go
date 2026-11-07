package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 351: while the hold key is down, every visible control with a key
// wears its key as caps, set on its top edge at its right.
func TestLayoutCaps(t *testing.T) {
	th := ui.NewTheme(1)
	window := city.RectAt(0, 0, 1000, 800)
	labels := func(caps []ui.KeyCap) []string {
		var out []string
		for _, c := range caps {
			out = append(out, c.Label.Text)
		}
		return out
	}

	t.Run("when controls with keys are laid", func(t *testing.T) {
		controls := []ui.Control{
			{Rect: city.RectAt(100, 100, 120, 30), Keys: []string{"space", "s"}},
			{Rect: city.RectAt(400, 300, 200, 40), Keys: []string{"v"}},
		}
		held := ui.LayoutCaps(th, controls, window, measure7)
		require.Len(t, held, 2)

		t.Run("it should lay one cap per press for each control", func(t *testing.T) {
			assert.Equal(t, [][]string{{"space", "s"}, {"v"}}, [][]string{labels(held[0].Caps), labels(held[1].Caps)})
		})

		t.Run("it should set each row's right edge on its control's right edge", func(t *testing.T) {
			for i, h := range held {
				assert.Equal(t, controls[i].Rect.Max.X, h.Caps[len(h.Caps)-1].Rect.Max.X, "control %d", i)
			}
		})

		t.Run("it should set each row's bottom half a grid over its control's top edge, clear of its label", func(t *testing.T) {
			for i, h := range held {
				assert.Equal(t, controls[i].Rect.Min.Y+th.Grid()/2, h.Caps[0].Rect.Max.Y, "control %d", i)
			}
		})

		t.Run("it should keep the control each row is for", func(t *testing.T) {
			assert.Equal(t, controls, []ui.Control{held[0].Control, held[1].Control})
		})
	})

	t.Run("when a control sits in the window's corner", func(t *testing.T) {
		held := ui.LayoutCaps(th, []ui.Control{{Rect: city.RectAt(0, 0, 10, 10), Keys: []string{"space", "s"}}}, window, measure7)
		require.Len(t, held, 1)

		t.Run("it should keep every cap inside the window", func(t *testing.T) {
			for _, c := range held[0].Caps {
				assert.Equal(t, window, window.Union(c.Rect), "cap %q at %v", c.Label.Text, c.Rect)
			}
		})
	})

	t.Run("when the controls are a row of buttons narrower than their caps", func(t *testing.T) {
		controls := []ui.Control{
			{Rect: city.RectAt(100, 100, 40, 30), Keys: []string{"enter"}},
			{Rect: city.RectAt(148, 100, 40, 30), Keys: []string{"space", "s"}},
			{Rect: city.RectAt(196, 100, 40, 30), Keys: []string{"space", "p"}},
		}
		held := ui.LayoutCaps(th, controls, window, measure7)
		require.Len(t, held, 3)

		t.Run("it should not overlap one button's caps with another's", func(t *testing.T) {
			for i := range held {
				for j := i + 1; j < len(held); j++ {
					for _, a := range held[i].Caps {
						for _, b := range held[j].Caps {
							assert.False(t, a.Rect.Overlaps(b.Rect), "%q at %v and %q at %v", a.Label.Text, a.Rect, b.Label.Text, b.Rect)
						}
					}
				}
			}
		})
	})

	t.Run("when a control has no keys", func(t *testing.T) {
		held := ui.LayoutCaps(th, []ui.Control{{Rect: city.RectAt(100, 100, 40, 30)}}, window, measure7)

		t.Run("it should place nothing for it", func(t *testing.T) {
			assert.Empty(t, held)
		})
	})
}
