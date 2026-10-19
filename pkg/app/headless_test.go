package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScreenSize(t *testing.T) {
	t.Run("when no window size is asked for", func(t *testing.T) {
		t.Run("it should match the default window", func(t *testing.T) {
			assert.Equal(t, [2]int{1100, 760}, screenSize(0, 0))
		})
	})

	t.Run("when a window size is asked for", func(t *testing.T) {
		t.Run("it should make the display that size, so the window is not clipped", func(t *testing.T) {
			assert.Equal(t, [2]int{1920, 1080}, screenSize(1920, 1080))
		})
	})
}
