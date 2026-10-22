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

// Item 199: the cuts left between two windows are laid a kind at a time
// and, among punches, a page at a time, so each page is one draw. Sorted
// by where each sprite's image happened to be allocated, the pages
// interleaved differently every run, and so did the frame's draw count.
func TestTheOrderCutsAreLaidIn(t *testing.T) {
	t.Run("when cuts are put in the order they are laid", func(t *testing.T) {
		a, b := ebiten.NewImage(8, 8), ebiten.NewImage(8, 8)
		defer a.Deallocate()
		defer b.Deallocate()
		punch := func(page *ebiten.Image, tag int) lightOp {
			return lightOp{punch: true, page: page, src: page.SubImage(page.Bounds()).(*ebiten.Image), from: tag}
		}
		ops := []lightOp{
			{cut: cutWords, from: 1}, punch(b, 2), {cut: cutStamp, from: 3}, punch(a, 4),
			{cut: cutRing, from: 5}, punch(b, 6), {cut: cutMesh, from: 7}, punch(a, 8),
		}
		orderCuts(ops)

		t.Run("and punches from two pages are noted between other cuts", func(t *testing.T) {
			t.Run("it should lay a kind at a time: punches, meshes, stamps, rings, then words", func(t *testing.T) {
				kinds := []int{}
				for _, op := range ops {
					kinds = append(kinds, cutOrder(op))
				}
				assert.Equal(t, []int{0, 0, 0, 0, 1, 2, 3, 4}, kinds)
			})

			t.Run("it should lay each page's punches together, the pages in the order first noted", func(t *testing.T) {
				tags := []int{}
				for _, op := range ops {
					tags = append(tags, op.from)
				}
				assert.Equal(t, []int{2, 6, 4, 8, 7, 3, 5, 1}, tags)
			})
		})

		// Shrunk, a punch is drawn from the mipmap its scale picks,
		// each a texture of its own, so a page's punches at two sizes
		// alternated between two textures.
		t.Run("and a page's punches are noted at two sizes, alternately", func(t *testing.T) {
			sized := func(scale float64, tag int) lightOp {
				op := punch(a, tag)
				op.geo.Scale(scale, scale)
				return op
			}
			ops := []lightOp{sized(0.5, 1), sized(0.25, 2), sized(0.5, 3), sized(0.25, 4)}
			orderCuts(ops)

			t.Run("it should lay the page's punches a size at a time, each size in the order noted", func(t *testing.T) {
				tags := []int{}
				for _, op := range ops {
					tags = append(tags, op.from)
				}
				assert.Equal(t, []int{1, 3, 2, 4}, tags)
			})
		})
	})
}
