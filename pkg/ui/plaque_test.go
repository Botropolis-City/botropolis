package ui

import (
	"testing"

	"botropolis.city/botropolis/pkg/city"
	"github.com/stretchr/testify/assert"
)

// A signage board stands in the world, so its face is not square to the
// screen: text painted on it has to lean into the board's plane the way
// the board does. Phase 21 item 38.

func measureFixed(s string, _ Size) (float64, float64) {
	return float64(len(s)) * 6, 10
}

func TestLayoutPlaque(t *testing.T) {
	face := city.RectAt(100, 50, 120, 30)

	t.Run("when the name fits the board's face", func(t *testing.T) {
		plaque, ok := LayoutPlaque("mullet", face, BoardLean, measureFixed)

		t.Run("it should accept it", func(t *testing.T) {
			assert.True(t, ok)
		})

		t.Run("it should lean the text into the board's plane", func(t *testing.T) {
			assert.Equal(t, BoardLean, plaque.Lean)
		})

		t.Run("it should centre the text across the face", func(t *testing.T) {
			w, _ := measureFixed("mullet", Small)
			assert.InDelta(t, face.Center().X, plaque.At.X+w*plaque.Scale/2, 0.001)
		})
	})

	t.Run("when the board is too small for legible text", func(t *testing.T) {
		t.Run("it should decline, leaving the name to the hover plate", func(t *testing.T) {
			_, ok := LayoutPlaque("mullet", city.RectAt(0, 0, 8, 2), BoardLean, measureFixed)
			assert.False(t, ok)
		})
	})

	t.Run("when there is no name", func(t *testing.T) {
		t.Run("it should decline rather than paint an empty board", func(t *testing.T) {
			_, ok := LayoutPlaque("", face, BoardLean, measureFixed)
			assert.False(t, ok)
		})
	})
}
