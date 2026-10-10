package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

func TestLayoutSparkline(t *testing.T) {
	th := ui.NewTheme(1)
	box := city.RectAt(10, 20, 100, 30)

	t.Run("when values are laid along a box", func(t *testing.T) {
		line := ui.LayoutSparkline(th, []float64{0, 5, 10, 5, 0}, box)

		t.Run("it should give one point per value", func(t *testing.T) {
			assert.Len(t, line.Points, 5)
		})

		t.Run("it should run from the left edge to the right", func(t *testing.T) {
			assert.Equal(t, 10.0, line.Points[0].X)
			assert.Equal(t, 110.0, line.Points[4].X)
		})

		t.Run("it should put the peak at the top and the floor at the bottom", func(t *testing.T) {
			assert.Equal(t, 20.0, line.Points[2].Y)
			assert.Equal(t, 50.0, line.Points[0].Y)
		})

		t.Run("it should mark the last point", func(t *testing.T) {
			assert.Equal(t, line.Points[4], line.Last)
		})

		t.Run("it should remember the peak", func(t *testing.T) {
			assert.Equal(t, 10.0, line.Peak)
		})
	})

	t.Run("when every value is zero", func(t *testing.T) {
		line := ui.LayoutSparkline(th, []float64{0, 0, 0}, box)

		t.Run("it should lie along the floor", func(t *testing.T) {
			for _, p := range line.Points {
				assert.Equal(t, 50.0, p.Y)
			}
		})
	})

	t.Run("when there is one value", func(t *testing.T) {
		line := ui.LayoutSparkline(th, []float64{3}, box)

		t.Run("it should be a single point at the right edge", func(t *testing.T) {
			require.Len(t, line.Points, 1)
			assert.Equal(t, 110.0, line.Points[0].X)
		})
	})

	t.Run("when there are no values", func(t *testing.T) {
		line := ui.LayoutSparkline(th, nil, box)

		t.Run("it should have no points", func(t *testing.T) {
			assert.Empty(t, line.Points)
		})
	})
}
