package render

import (
	"image"
	"image/color"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/shapes"
	"botropolis.city/botropolis/pkg/ui"
)

// inkCovers is whether an inked box holds want and stays within slack of
// it: what it would have drawn, and not much more.
func inkCovers(box, want image.Rectangle, slack int) bool {
	return want.In(box) && box.In(want.Inset(-slack))
}

// Item 193: the dry pass records what every drawable would draw, the
// box it would ink and a hash of it, so the still city can be composed
// once and replayed only where something moves.
func TestInk(t *testing.T) {
	sprite := ebiten.NewImage(10, 20)
	page := ebiten.NewImage(64, 64)
	other := ebiten.NewImage(64, 64)
	f, err := newFaces(ui.NewTheme(1))
	require.NoError(t, err)

	t.Run("when a sprite is drawn dry", func(t *testing.T) {
		l := newDrawList()
		l.begin()
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		op.GeoM.Scale(2, 2)
		op.GeoM.Translate(100.5, 50)
		l.Image(l.dry, sprite, op)
		got := l.end()

		t.Run("it should ink the sprite's transformed box, padded for linear filtering", func(t *testing.T) {
			assert.Equal(t, image.Rect(100-inkPad, 50-inkPad, 121+inkPad, 90+inkPad), got.box)
		})

		t.Run("it should record one command", func(t *testing.T) {
			assert.Equal(t, 1, got.to-got.from)
		})
	})

	t.Run("when a shape is drawn dry through the painter", func(t *testing.T) {
		tri := []city.Point{{X: 10, Y: 10}, {X: 30, Y: 10}, {X: 10, Y: 40}}
		for _, c := range []struct {
			name string
			draw func(p *shapes.Painter, dst *ebiten.Image)
			want image.Rectangle
		}{
			{"a mesh", func(p *shapes.Painter, dst *ebiten.Image) { p.Fill(dst, tri, color.NRGBA{A: 255}) }, image.Rect(10, 10, 30, 40)},
			{"a disc", func(p *shapes.Painter, dst *ebiten.Image) {
				p.Disc(dst, city.Point{X: 50, Y: 50}, 6, color.NRGBA{A: 255})
			}, image.Rect(44, 44, 56, 56)},
			{"a line", func(p *shapes.Painter, dst *ebiten.Image) { p.Line(dst, 0, 5, 40, 5, 2, color.NRGBA{A: 255}) }, image.Rect(0, 4, 40, 6)},
			{"a box", func(p *shapes.Painter, dst *ebiten.Image) { p.Box(dst, 5, 6, 7, 8, color.White) }, image.Rect(5, 6, 12, 14)},
		} {
			t.Run("and it is "+c.name, func(t *testing.T) {
				l := newDrawList()
				p := &shapes.Painter{Record: l}
				l.begin()
				c.draw(p, l.dry)
				got := l.end()

				t.Run("it should ink a box round the shape", func(t *testing.T) {
					assert.True(t, inkCovers(got.box, c.want, 2*inkPad+2), "inked %v for %v", got.box, c.want)
				})
			})
		}
	})

	t.Run("when words are drawn dry", func(t *testing.T) {
		l := newDrawList()
		op := &text.DrawOptions{}
		op.GeoM.Translate(200, 100)
		w, h := f.Measure("Botropolis", ui.Small)
		l.begin()
		l.words(l.dry, "Botropolis", f.Face(ui.Small), op)
		got := l.end()

		t.Run("it should ink a box round the words", func(t *testing.T) {
			want := image.Rect(200, 100, 200+int(w), 100+int(h))
			assert.True(t, inkCovers(got.box, want, 2*inkPad+2), "inked %v for %v", got.box, want)
		})
	})

	t.Run("when a draw goes to an image that is not the dry one", func(t *testing.T) {
		l := newDrawList()
		dst := ebiten.NewImage(8, 8)
		l.begin()
		l.Image(dst, sprite, &ebiten.DrawImageOptions{})
		got := l.end()

		t.Run("it should record nothing", func(t *testing.T) {
			assert.Equal(t, 0, got.to-got.from)
		})
	})

	t.Run("when the same draws are made twice", func(t *testing.T) {
		draw := func(l *drawList) inked {
			l.begin()
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(3, 4)
			l.Image(l.dry, page.SubImage(image.Rect(0, 0, 8, 8)).(*ebiten.Image), op)
			l.words(l.dry, "a", f.Face(ui.Small), &text.DrawOptions{})
			return l.end()
		}
		a, b := draw(newDrawList()), draw(newDrawList())

		t.Run("it should hash them equal", func(t *testing.T) {
			assert.Equal(t, a.hash, b.hash)
		})
	})

	t.Run("when one parameter of a draw differs", func(t *testing.T) {
		base := func(l *drawList, src *ebiten.Image, geo ebiten.GeoM, scale float32, words string) inked {
			l.begin()
			op := &ebiten.DrawImageOptions{GeoM: geo}
			op.ColorScale.ScaleAlpha(scale)
			l.Image(l.dry, src.SubImage(image.Rect(0, 0, 8, 8)).(*ebiten.Image), op)
			l.words(l.dry, words, f.Face(ui.Small), &text.DrawOptions{})
			return l.end()
		}
		var moved ebiten.GeoM
		moved.Translate(1, 0)
		want := base(newDrawList(), page, ebiten.GeoM{}, 1, "a")
		for _, c := range []struct {
			name string
			got  inked
		}{
			{"the source page", base(newDrawList(), other, ebiten.GeoM{}, 1, "a")},
			{"the transform", base(newDrawList(), page, moved, 1, "a")},
			{"the colour", base(newDrawList(), page, ebiten.GeoM{}, 0.5, "a")},
			{"the words", base(newDrawList(), page, ebiten.GeoM{}, 1, "b")},
		} {
			t.Run("and it is "+c.name, func(t *testing.T) {
				t.Run("it should hash them apart", func(t *testing.T) {
					assert.NotEqual(t, want.hash, c.got.hash)
				})
			})
		}
	})

	t.Run("when two meshes differ in a vertex", func(t *testing.T) {
		mesh := func(x float32) inked {
			l := newDrawList()
			l.begin()
			v := []ebiten.Vertex{{DstX: 0, DstY: 0}, {DstX: x, DstY: 0}, {DstX: 0, DstY: 5}}
			l.Mesh(l.dry, v, []uint32{0, 1, 2}, page, &ebiten.DrawTrianglesOptions{}, shapes.MeshKey{})
			return l.end()
		}

		t.Run("it should hash them apart", func(t *testing.T) {
			assert.NotEqual(t, mesh(5).hash, mesh(6).hash)
		})
	})
}
