package ui_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/ui"
)

func TestLayoutFinder(t *testing.T) {
	th := ui.NewTheme(1)

	t.Run("when the finder is laid out in a 1280×800 window", func(t *testing.T) {
		p := ui.LayoutFinder(th, 1280, 800, ui.FinderInput{Query: "deploy"}, measure7)

		t.Run("it should centre the panel across the window", func(t *testing.T) {
			assert.Equal(t, 640.0, p.Rect.Center().X)
		})

		t.Run("it should hang the panel from the top third, like a command palette", func(t *testing.T) {
			assert.True(t, p.Rect.Min.Y > 0 && p.Rect.Min.Y < 800.0/3, "top at %v", p.Rect.Min.Y)
		})

		t.Run("it should put the field on the panel's first line", func(t *testing.T) {
			assert.Equal(t, p.Rect.Min.Y+2*th.Grid(), p.Field.Min.Y)
		})

		t.Run("it should draw with the plates' palette, radius and grid", func(t *testing.T) {
			type look struct {
				Fill   any
				Radius float64
				Pad    float64
			}
			assert.Equal(t, look{th.Palette.Panel, th.Radius(), 2 * th.Grid()}, look{p.Fill, p.Radius, p.Pad})
		})
	})

	t.Run("when the window is narrower than the panel", func(t *testing.T) {
		p := ui.LayoutFinder(th, 300, 800, ui.FinderInput{Query: "deploy"}, measure7)

		t.Run("it should stay inside the window", func(t *testing.T) {
			assert.True(t, p.Rect.Width() > 0 && p.Rect.Min.X >= 0 && p.Rect.Max.X <= 300, "panel %v", p.Rect)
		})
	})

	t.Run("when the query is wider than the field", func(t *testing.T) {
		query := strings.Repeat("abcdefghij", 30)
		p := ui.LayoutFinder(th, 1280, 800, ui.FinderInput{Query: query, Caret: len(query)}, measure7)

		t.Run("it should keep the caret in view", func(t *testing.T) {
			assert.True(t, p.Caret.Width() > 0 && p.Caret.Min.X >= p.Field.Min.X && p.Caret.Max.X <= p.Field.Max.X, "caret %v in field %v", p.Caret, p.Field)
		})
	})
}
