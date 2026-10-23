package shapes

import (
	"image/color"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// meshCount is a Recorder that draws what it is given, counts the
// meshes and keeps their triangles in the order drawn, each mesh's
// indices moved past the vertices before it.
type meshCount struct {
	meshes   int
	vertices []ebiten.Vertex
	indices  []uint32
}

func (m *meshCount) Image(dst, img *ebiten.Image, op *ebiten.DrawImageOptions) {
	dst.DrawImage(img, op)
}

func (m *meshCount) Mesh(dst *ebiten.Image, vertices []ebiten.Vertex, indices []uint32, src *ebiten.Image, op *ebiten.DrawTrianglesOptions, _ MeshKey) {
	m.meshes++
	for _, i := range indices {
		m.indices = append(m.indices, i+uint32(len(m.vertices)))
	}
	m.vertices = append(m.vertices, vertices...)
	dst.DrawTriangles32(vertices, indices, src, op)
}

func (m *meshCount) Ring(dst *ebiten.Image, pts []city.Point, op *vector.DrawPathOptions) {}

// Item 207: the cracks on an aged lot and the dashes of a group line
// were drawn a line at a time, a mesh each, and on the real history
// recording them was most of what an unfocused city's frame spent.
// Drawn together they are one mesh, and paint and cut what each drawn
// alone would.
func TestPainterLines(t *testing.T) {
	half := color.NRGBA{R: 200, G: 100, B: 50, A: 128}
	// Two segments of a crack meeting at a point, and a third crossing
	// them, so overlapping soft edges blend as they did one by one.
	segs := []city.Point{{X: 4, Y: 4}, {X: 30, Y: 12}, {X: 30, Y: 12}, {X: 20, Y: 34}, {X: 6, Y: 30}, {X: 36, Y: 8}}
	const width = 2.5
	alone := func(p *Painter, dst *ebiten.Image) {
		for i := 0; i+1 < len(segs); i += 2 {
			p.Line(dst, segs[i].X, segs[i].Y, segs[i+1].X, segs[i+1].Y, width, half)
		}
	}
	together := func(p *Painter, dst *ebiten.Image) { p.Lines(dst, segs, width, half) }

	t.Run("when lines are drawn together", func(t *testing.T) {
		count := &meshCount{}
		dst := ebiten.NewImage(40, 40)
		defer dst.Deallocate()
		together(&Painter{Record: count}, dst)
		each := &meshCount{}
		alone(&Painter{Record: each}, dst)

		t.Run("it should draw them in one mesh", func(t *testing.T) {
			assert.Equal(t, 1, count.meshes)
		})

		// The same triangles in the same order blend to the same pixels.
		t.Run("it should draw the triangles drawing each alone draws, in order", func(t *testing.T) {
			assert.Equal(t, []any{each.vertices, each.indices}, []any{count.vertices, count.indices})
		})
	})

	t.Run("when lines are drawn together under a cut hook", func(t *testing.T) {
		cut := func(paint func(p *Painter, dst *ebiten.Image)) *cutLog {
			dst := ebiten.NewImage(40, 40)
			defer dst.Deallocate()
			log := &cutLog{}
			paint(&Painter{Cut: log}, dst)
			return log
		}

		t.Run("it should cut what cutting each alone cuts", func(t *testing.T) {
			cuts := func(l *cutLog) []any { return []any{l.meshes, l.discs, l.discAlpha, l.geoms} }
			assert.Equal(t, cuts(cut(alone)), cuts(cut(together)))
		})
	})

	t.Run("when no lines are drawn together", func(t *testing.T) {
		count := &meshCount{}
		dst := ebiten.NewImage(4, 4)
		defer dst.Deallocate()
		(&Painter{Record: count}).Lines(dst, nil, width, half)

		t.Run("it should draw nothing", func(t *testing.T) {
			assert.Zero(t, count.meshes)
		})
	})
}
