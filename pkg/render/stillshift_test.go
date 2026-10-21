package render

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/shapes"
)

// Item 193: a pan by whole pixels moves the standing city rather than
// composing it again: what each still drawable paints is hashed relative
// to the camera, the layers are shifted, and only the strips the move
// exposes are composed.
func TestStillShift(t *testing.T) {
	sprite := ebiten.NewImage(10, 20)
	drawn := func(origin, at city.Point) inked {
		l := newDrawList()
		l.origin = origin
		l.begin()
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(at.X, at.Y)
		l.Image(l.dry, sprite, op)
		v := []ebiten.Vertex{{DstX: float32(at.X), DstY: float32(at.Y)}, {DstX: float32(at.X) + 4, DstY: float32(at.Y)}, {DstX: float32(at.X), DstY: float32(at.Y) + 4}}
		l.Mesh(l.dry, v, []uint32{0, 1, 2}, sprite, &ebiten.DrawTrianglesOptions{}, shapes.MeshKey{})
		return l.end()
	}
	before := drawn(city.Point{X: 10.25, Y: 3}, city.Point{X: 100.5, Y: 50.25})

	t.Run("when the camera moves by whole pixels", func(t *testing.T) {
		after := drawn(city.Point{X: 13.25, Y: 1}, city.Point{X: 103.5, Y: 48.25})

		t.Run("it should keep every still drawable's hash", func(t *testing.T) {
			assert.Equal(t, before.hash, after.hash)
		})
	})

	t.Run("when the camera moves by whole pixels, and a transform's scale comes out a few bits apart", func(t *testing.T) {
		scaled := func(origin city.Point, sx float64) uint64 {
			l := newDrawList()
			l.origin = origin
			l.begin()
			op := &ebiten.DrawImageOptions{}
			op.GeoM.SetElement(0, 0, sx)
			op.GeoM.SetElement(0, 2, origin.X+5)
			l.Image(l.dry, sprite, op)
			return l.end().hash
		}

		t.Run("it should keep the hash", func(t *testing.T) {
			assert.Equal(t, scaled(city.Point{X: 616}, 1.4705882352940307), scaled(city.Point{X: 604}, 1.4705882352941444))
		})
	})

	t.Run("when two meshes were built from the same painted shape, and their float32 vertices round a hair apart", func(t *testing.T) {
		key := shapes.MeshKey{Points: []city.Point{{X: 1020.3, Y: 50.7}, {X: 1023.9, Y: 80.1}}, Values: []float64{2, 2, 1.3, 0, 0, 0, 0, 255}}
		mesh := func(x float32) uint64 {
			l := newDrawList()
			l.begin()
			v := []ebiten.Vertex{{DstX: x, DstY: 50}, {DstX: x + 4, DstY: 50}, {DstX: x, DstY: 54}}
			l.Mesh(l.dry, v, []uint32{0, 1, 2}, sprite, &ebiten.DrawTrianglesOptions{}, key)
			return l.end().hash
		}

		t.Run("it should hash them by the shape, alike", func(t *testing.T) {
			assert.Equal(t, mesh(1020.3026), mesh(1020.3029))
		})
	})

	t.Run("when a box's edge falls on a whole pixel, give or take rounding in the last bits", func(t *testing.T) {
		t.Run("it should round it to the same pixel", func(t *testing.T) {
			assert.Equal(t, pixelBox(616-1e-12, 3, 700+1e-12, 9), pixelBox(616+1e-12, 3, 700-1e-12, 9))
		})
	})

	t.Run("when a drawable moves and the camera does not", func(t *testing.T) {
		after := drawn(city.Point{X: 10.25, Y: 3}, city.Point{X: 101.5, Y: 50.25})

		t.Run("it should hash it apart", func(t *testing.T) {
			assert.NotEqual(t, before.hash, after.hash)
		})
	})

	frame := image.Rect(0, 0, 100, 60)
	for _, c := range []struct {
		name     string
		from, to city.Point
		want     []image.Rectangle
		whole    bool
	}{
		{"when the camera moves right by 3 pixels", city.Point{X: 10, Y: 5}, city.Point{X: 13, Y: 5}, []image.Rectangle{image.Rect(0, 0, 3, 60)}, false},
		{"when it moves left by 3 pixels", city.Point{X: 10, Y: 5}, city.Point{X: 7, Y: 5}, []image.Rectangle{image.Rect(97, 0, 100, 60)}, false},
		{"when it moves down by 2 pixels", city.Point{X: 10, Y: 5}, city.Point{X: 10, Y: 7}, []image.Rectangle{image.Rect(0, 0, 100, 2)}, false},
		{"when it moves up by 2 pixels", city.Point{X: 10, Y: 5}, city.Point{X: 10, Y: 3}, []image.Rectangle{image.Rect(0, 58, 100, 60)}, false},
		{"when it moves diagonally", city.Point{X: 10, Y: 5}, city.Point{X: 13, Y: 3}, []image.Rectangle{image.Rect(0, 0, 3, 60), image.Rect(3, 58, 100, 60)}, false},
		{"when it moves by part of a pixel", city.Point{X: 10, Y: 5}, city.Point{X: 10.5, Y: 5}, nil, true},
		{"when it moves further than the frame", city.Point{X: 10, Y: 5}, city.Point{X: 210, Y: 5}, nil, true},
		{"when it does not move", city.Point{X: 10.25, Y: 5}, city.Point{X: 10.25, Y: 5}, nil, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			k, ok := wholeShift(c.from, c.to, frame)

			t.Run("it should say whether the layers can be shifted", func(t *testing.T) {
				assert.Equal(t, !c.whole, ok)
			})

			if !c.whole {
				t.Run("it should compose only the strips the move exposes", func(t *testing.T) {
					assert.ElementsMatch(t, c.want, exposed(frame, k))
				})
			}
		})
	}

	t.Run("when the still set moves with the camera", func(t *testing.T) {
		still := func(hash uint64, x int) entry {
			return entry{inked: inked{hash: hash, box: image.Rect(x, 0, x+10, 10)}}
		}
		set := newStillSet([]entry{still(1, 0), still(2, 30)})
		set.shift(image.Pt(5, 0))

		t.Run("it should find nothing changed where everything moved by the shift", func(t *testing.T) {
			assert.Empty(t, set.changes(nil, []entry{still(1, 5), still(2, 35)}))
		})
	})
}
