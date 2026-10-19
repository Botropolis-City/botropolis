package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGameHoldsGLOnce(t *testing.T) {
	t.Run("when the game updates for the first time", func(t *testing.T) {
		held := 0
		g := &Game{record: "/frames", ticks: 7, painted: 6, holdGL: func() { held++ }}
		_ = g.Update()

		t.Run("it should hold the GL libraries", func(t *testing.T) {
			assert.Equal(t, 1, held)
		})

		t.Run("and it updates again", func(t *testing.T) {
			_ = g.Update()

			t.Run("it should not hold them a second time", func(t *testing.T) {
				assert.Equal(t, 1, held)
			})
		})
	})
}
