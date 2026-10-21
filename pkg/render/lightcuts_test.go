package render

import (
	"image/color"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 195: whatever is painted in front of a lit window, and not only a
// sprite, cuts it out of the night's lights.
func TestPaintedShapesCutTheLights(t *testing.T) {
	ink := color.NRGBA{R: 40, G: 40, B: 40, A: 255}
	page := ebiten.NewImage(4, 4)
	pane := func(x, y float32) []ebiten.Vertex {
		return []ebiten.Vertex{
			{DstX: x, DstY: y, ColorA: 1}, {DstX: x + 10, DstY: y, ColorA: 1},
			{DstX: x + 10, DstY: y + 10, ColorA: 1}, {DstX: x, DstY: y + 10, ColorA: 1},
		}
	}
	night := func(lit bool) *Game {
		g := &Game{}
		g.beginLights(200, 200)
		if lit {
			g.noteLights(pane(20, 20), []uint16{0, 1, 2, 0, 2, 3}, page)
		}
		return g
	}
	screen := ebiten.NewImage(200, 200)

	t.Run("when a building's windows are lit", func(t *testing.T) {
		t.Run("and a shape is painted over them", func(t *testing.T) {
			g := night(true)
			g.fillRect(screen, 15, 15, 20, 20, ink)

			t.Run("it should note a cut", func(t *testing.T) {
				assert.Equal(t, 1, g.cuts)
			})
		})

		t.Run("and two shapes are painted over them, one after the other", func(t *testing.T) {
			g := night(true)
			g.fillRect(screen, 15, 15, 20, 20, ink)
			g.fillRect(screen, 18, 22, 10, 6, ink)

			t.Run("it should lay their meshes as one cut", func(t *testing.T) {
				assert.Equal(t, 1, countCutOps(g))
			})
		})

		t.Run("and a disc is painted over them", func(t *testing.T) {
			g := night(true)
			g.fillCircle(screen, 25, 25, 6, ink)

			t.Run("it should note a cut", func(t *testing.T) {
				assert.Equal(t, 1, g.cuts)
			})
		})

		t.Run("and a glow is drawn over them", func(t *testing.T) {
			g := night(true)
			g.paint.DiscBlend(screen, city.Point{X: 25, Y: 25}, 6, ink, ebiten.BlendLighter)

			t.Run("it should note nothing", func(t *testing.T) {
				assert.Zero(t, g.cuts)
			})
		})

		t.Run("and a shape is painted far from them", func(t *testing.T) {
			g := night(true)
			g.fillRect(screen, 150, 150, 20, 20, ink)

			t.Run("it should note nothing", func(t *testing.T) {
				assert.Zero(t, g.cuts)
			})
		})

		t.Run("and a shape is painted over them while painting is uncut", func(t *testing.T) {
			g := night(true)
			g.paintUncut = true
			g.fillRect(screen, 15, 15, 20, 20, ink)

			t.Run("it should note nothing", func(t *testing.T) {
				assert.Zero(t, g.cuts)
			})
		})

		t.Run("and the lights have been laid", func(t *testing.T) {
			g := night(true)
			g.endLights(screen)
			g.fillRect(screen, 15, 15, 20, 20, ink)

			t.Run("it should note nothing painted after them", func(t *testing.T) {
				assert.Zero(t, g.cuts)
			})
		})
	})

	t.Run("when no window is lit yet and a shape is painted", func(t *testing.T) {
		g := night(false)
		g.fillRect(screen, 15, 15, 20, 20, ink)

		t.Run("it should note nothing", func(t *testing.T) {
			assert.Zero(t, g.cuts)
		})
	})
}

func countCutOps(g *Game) int {
	n := 0
	for _, op := range g.lightOps {
		if op.cut != cutNone {
			n++
		}
	}
	return n
}

// Painted words are cut out too: a sign's letters over a lit pane.
func TestPaintedWordsCutTheLights(t *testing.T) {
	g := landmarkLabelGame(t)
	page := ebiten.NewImage(4, 4)
	g.beginLights(200, 200)
	g.noteLights([]ebiten.Vertex{
		{DstX: 0, DstY: 0, ColorA: 1}, {DstX: 200, DstY: 0, ColorA: 1},
		{DstX: 200, DstY: 200, ColorA: 1}, {DstX: 0, DstY: 200, ColorA: 1},
	}, []uint16{0, 1, 2, 0, 2, 3}, page)
	g.sign(ebiten.NewImage(200, 200), ui.Sign{Text: "orchard", At: city.Point{X: 20, Y: 20}, Scale: 1}, color.NRGBA{A: 255})

	t.Run("when a word is painted over a lit pane", func(t *testing.T) {
		t.Run("it should note a cut", func(t *testing.T) {
			assert.Equal(t, 1, g.cuts)
		})
	})
}
