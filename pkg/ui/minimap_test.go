package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

func TestLayoutMinimap(t *testing.T) {
	th := ui.NewTheme(1)

	t.Run("when the window is wide enough", func(t *testing.T) {
		box, ok := ui.LayoutMinimap(th, 800, 600, 32)

		t.Run("it should be shown", func(t *testing.T) {
			assert.True(t, ok)
		})

		t.Run("it should sit in the bottom-right corner above the footer", func(t *testing.T) {
			assert.Equal(t, city.Point{X: 800 - 16, Y: 600 - 32 - 16}, box.Max)
		})

		t.Run("it should be 160 by 96", func(t *testing.T) {
			assert.Equal(t, city.Point{X: 160, Y: 96}, box.Size())
		})
	})

	t.Run("when the window is narrower than three minimaps", func(t *testing.T) {
		_, ok := ui.LayoutMinimap(th, 400, 600, 32)

		t.Run("it should be hidden", func(t *testing.T) {
			assert.False(t, ok)
		})
	})
}
