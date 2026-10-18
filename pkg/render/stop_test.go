package render

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
)

// Item 114: a signal ends the loop rather than the process, so Run
// returns and whoever started it can clean up after it.
func TestGameStop(t *testing.T) {
	t.Run("when the game is asked to stop", func(t *testing.T) {
		g := &Game{}
		g.Stop()

		t.Run("it should end the loop with ebiten.Termination", func(t *testing.T) {
			assert.Equal(t, ebiten.Termination, g.Update())
		})

		t.Run("and it is recording between paints", func(t *testing.T) {
			g := &Game{record: "/frames", ticks: 7, painted: 6}
			g.Stop()

			t.Run("it should end the loop with ebiten.Termination", func(t *testing.T) {
				assert.Equal(t, ebiten.Termination, g.Update())
			})
		})
	})

	t.Run("when the game is not asked to stop, and it is recording between paints", func(t *testing.T) {
		g := &Game{record: "/frames", ticks: 7, painted: 6}

		t.Run("it should keep running", func(t *testing.T) {
			assert.NoError(t, g.Update())
		})
	})
}
