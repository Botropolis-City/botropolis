package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/ui"
)

func TestFaces(t *testing.T) {
	t.Run("when measuring at 1x", func(t *testing.T) {
		f, err := newFaces(ui.NewTheme(1))
		require.NoError(t, err)
		short, _ := f.Measure("city", ui.Body)
		long, _ := f.Measure("city hall", ui.Body)

		t.Run("it should measure a longer string wider", func(t *testing.T) {
			assert.Greater(t, long, short)
		})

		t.Run("it should measure the display size taller than the small size", func(t *testing.T) {
			_, small := f.Measure("city", ui.Small)
			_, display := f.Measure("city", ui.Display)
			assert.Greater(t, display, small)
		})

		t.Run("it should measure nothing as no width", func(t *testing.T) {
			w, _ := f.Measure("", ui.Body)
			assert.Zero(t, w)
		})

		t.Run("it should set figures tabular, so numbers line up", func(t *testing.T) {
			ones, _ := f.Measure("111", ui.Body)
			eights, _ := f.Measure("888", ui.Body)
			assert.Equal(t, eights, ones)
		})
	})

	t.Run("when measuring at 2x", func(t *testing.T) {
		one, err := newFaces(ui.NewTheme(1))
		require.NoError(t, err)
		two, err := newFaces(ui.NewTheme(2))
		require.NoError(t, err)
		w1, h1 := one.Measure("2 working", ui.Body)
		w2, h2 := two.Measure("2 working", ui.Body)

		t.Run("it should measure the same string twice as wide", func(t *testing.T) {
			assert.InDelta(t, 2*w1, w2, 1)
		})

		t.Run("it should measure the same string twice as tall", func(t *testing.T) {
			assert.InDelta(t, 2*h1, h2, 1)
		})
	})
}
