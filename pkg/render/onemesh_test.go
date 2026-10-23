package render

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// Item 207: a dashed group line and an aged lot's cracks recorded a
// mesh for every dash and every crack's segment, and on the real
// history recording them was most of an unfocused city's frame. Each
// is one mesh now; pkg/shapes' TestPainterLines holds that one mesh
// draws what the lines drawn one by one drew.
func TestOneMeshPerLine(t *testing.T) {
	ink := color.NRGBA{R: 90, G: 80, B: 70, A: 200}
	cracks := [][]city.Point{
		{{X: 0, Y: 0}, {X: 10, Y: 4}, {X: 16, Y: 12}, {X: 20, Y: 20}},
		{{X: 40, Y: 0}, {X: 44, Y: 8}},
	}

	for _, c := range []struct {
		name string
		draw func(g *Game, l *drawList)
	}{
		{"when a group line is dashed", func(g *Game, l *drawList) {
			g.dashed(l.dry, city.Point{X: 0, Y: 0}, city.Point{X: 200, Y: 0}, 3, 6, ink)
		}},
		{"when an aged lot's cracks are drawn", func(g *Game, l *drawList) {
			g.cracks(l.dry, city.NewCamera(), cracks, 1.2, ink)
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			g := &Game{}
			l := g.recorder()
			l.begin()
			c.draw(g, l)
			got := l.end()

			t.Run("it should record one mesh", func(t *testing.T) {
				assert.Equal(t, 1, got.to-got.from)
			})
		})
	}
}
