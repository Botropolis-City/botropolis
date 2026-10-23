package render

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
)

// Item 162: a pan shifts the layers through a frame-sized scratch image,
// which was then kept for the life of the city: 16 MiB at 1920×1200,
// where every texture is rounded up to 2048 pixels a side.
func TestPanScratchLetGo(t *testing.T) {
	panned := func() *Game {
		g := &Game{}
		layer := ebiten.NewImage(8, 8)
		g.shiftLayer(&layer, image.Pt(1, 0))
		return g
	}

	t.Run("when a layer has been shifted through the scratch", func(t *testing.T) {
		t.Run("and no layer is shifted for scratchHold frames", func(t *testing.T) {
			g := panned()
			for range scratchHold {
				g.restScratch()
			}

			t.Run("it should hold no scratch image", func(t *testing.T) {
				assert.Nil(t, g.still.scratch)
			})
		})

		t.Run("and a layer is shifted again within scratchHold frames", func(t *testing.T) {
			g := panned()
			layer := ebiten.NewImage(8, 8)
			for i := range scratchHold {
				g.restScratch()
				if i == scratchHold/2 {
					g.shiftLayer(&layer, image.Pt(0, 1))
				}
			}

			t.Run("it should keep the scratch image", func(t *testing.T) {
				assert.NotNil(t, g.still.scratch)
			})
		})
	})
}
