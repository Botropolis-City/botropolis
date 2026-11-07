package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 351: the raised surface is the panel's tone a step lighter, so a
// menu reads as on top of the card without a new hue.
func TestRaisedSurface(t *testing.T) {
	p, r := ui.DefaultPalette.Panel, ui.DefaultPalette.Raised

	t.Run("it should be lighter than the panel by one step in every channel", func(t *testing.T) {
		step := int(r.R) - int(p.R)
		assert.True(t, step > 0 && int(r.G)-int(p.G) == step && int(r.B)-int(p.B) == step, "panel %v, raised %v", p, r)
	})

	t.Run("it should be opaque", func(t *testing.T) {
		assert.Equal(t, uint8(0xff), r.A)
	})
}

// Item 351: a menu opened from a button continues from it: under it,
// left edges aligned, raised over the card, with a notch pointing at
// the button. One opened at a point has no notch.
func TestLayoutMenuUnder(t *testing.T) {
	th := ui.NewTheme(1)
	grid := th.Grid()
	bounds := city.RectAt(0, 0, 1000, 800)
	columns := []ui.MenuColumn{
		{Title: "Session", Key: "s", Rows: []ui.MenuRow{{Label: "attach", Key: "a"}, {Label: "stop", Key: "s"}}},
		{Title: "Project", Key: "p", Rows: []ui.MenuRow{{Label: "reveal folder", Key: "f"}}},
	}
	away := city.Point{X: -1, Y: -1}

	t.Run("when it opens under a button", func(t *testing.T) {
		button := city.RectAt(200, 300, 80, 24)
		m := ui.LayoutMenuUnder(th, columns, ui.NoMenuFocus, away, button, bounds, measure7)

		t.Run("it should put its left edge on the button's", func(t *testing.T) {
			assert.Equal(t, button.Min.X, m.Rect.Min.X)
		})

		t.Run("it should put its top a grid under the button", func(t *testing.T) {
			assert.Equal(t, button.Max.Y+grid, m.Rect.Min.Y)
		})

		t.Run("it should lay a notch on its top edge, its tip under the button's centre", func(t *testing.T) {
			require.True(t, m.HasNotch)
			assert.Equal(t, []float64{m.Rect.Min.Y, m.Rect.Min.Y, button.Center().X}, []float64{m.Notch[0].Y, m.Notch[2].Y, m.Notch[1].X})
		})

		t.Run("it should point the notch's tip up at the button", func(t *testing.T) {
			assert.True(t, m.Notch[1].Y < m.Rect.Min.Y && m.Notch[1].Y >= button.Max.Y, "tip %v", m.Notch[1])
		})

		t.Run("it should fill with the raised surface", func(t *testing.T) {
			assert.Equal(t, th.Palette.Raised, m.Fill)
		})

		t.Run("it should lay a shadow under the plate, offset down", func(t *testing.T) {
			assert.True(t, m.Shadow.Min.Y > m.Rect.Min.Y && m.Shadow.Width() >= m.Rect.Width(), "shadow %v, plate %v", m.Shadow, m.Rect)
		})
	})

	t.Run("when the button is near the window's right edge", func(t *testing.T) {
		button := city.RectAt(960, 300, 36, 24)
		m := ui.LayoutMenuUnder(th, columns, ui.NoMenuFocus, away, button, bounds, measure7)

		t.Run("it should move left to stay inside", func(t *testing.T) {
			assert.LessOrEqual(t, m.Rect.Max.X, bounds.Max.X)
		})

		t.Run("it should keep the notch's tip under the button, a radius in from the corner", func(t *testing.T) {
			assert.True(t, m.Notch[1].X >= button.Min.X && m.Notch[2].X <= m.Rect.Max.X-th.Radius(), "notch %v, button %v, menu %v", m.Notch, button, m.Rect)
		})
	})

	t.Run("when there is no room under the button", func(t *testing.T) {
		button := city.RectAt(200, 760, 80, 24)
		m := ui.LayoutMenuUnder(th, columns, ui.NoMenuFocus, away, button, bounds, measure7)

		t.Run("it should open above it", func(t *testing.T) {
			assert.Equal(t, button.Min.Y-grid, m.Rect.Max.Y)
		})

		t.Run("it should lay the notch on its bottom edge, its tip down at the button", func(t *testing.T) {
			assert.True(t, m.Notch[0].Y == m.Rect.Max.Y && m.Notch[1].Y > m.Rect.Max.Y && m.Notch[1].Y <= button.Min.Y, "notch %v", m.Notch)
		})
	})

	t.Run("when it opens at a point", func(t *testing.T) {
		m := ui.LayoutMenu(th, columns, ui.NoMenuFocus, away, city.Point{X: 100, Y: 100}, bounds, measure7)

		t.Run("it should lay no notch", func(t *testing.T) {
			assert.False(t, m.HasNotch)
		})

		t.Run("it should still be raised", func(t *testing.T) {
			assert.Equal(t, th.Palette.Raised, m.Fill)
		})

		t.Run("it should still lay its shadow", func(t *testing.T) {
			assert.Greater(t, m.Shadow.Area(), 0.0)
		})
	})
}
