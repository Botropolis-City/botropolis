package render

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Item 193: what moves is drawn again inside rectangles, each a merge
// of the movers' boxes that meet, clipped to the frame.
func TestDirtyRects(t *testing.T) {
	frame := image.Rect(0, 0, 100, 100)
	for _, c := range []struct {
		name  string
		boxes []image.Rectangle
		want  []image.Rectangle
	}{
		{"when two mover boxes overlap", []image.Rectangle{image.Rect(10, 10, 30, 30), image.Rect(20, 20, 40, 40)}, []image.Rectangle{image.Rect(10, 10, 40, 40)}},
		{"when two mover boxes touch", []image.Rectangle{image.Rect(10, 10, 20, 20), image.Rect(20, 10, 30, 20)}, []image.Rectangle{image.Rect(10, 10, 30, 20)}},
		{"when two mover boxes are apart", []image.Rectangle{image.Rect(0, 0, 10, 10), image.Rect(50, 50, 60, 60)}, []image.Rectangle{image.Rect(0, 0, 10, 10), image.Rect(50, 50, 60, 60)}},
		{"when a merge reaches a third box", []image.Rectangle{image.Rect(0, 0, 10, 10), image.Rect(50, 0, 60, 10), image.Rect(5, 0, 55, 5)}, []image.Rectangle{image.Rect(0, 0, 60, 10)}},
		{"when a box runs off the frame", []image.Rectangle{image.Rect(90, -5, 120, 10)}, []image.Rectangle{image.Rect(90, 0, 100, 10)}},
		{"when a box is wholly off the frame", []image.Rectangle{image.Rect(200, 200, 210, 210)}, nil},
		{"when a box is empty", []image.Rectangle{{}}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Run("it should draw again inside exactly the merged rectangles", func(t *testing.T) {
				assert.ElementsMatch(t, c.want, dirtyRects(nil, c.boxes, frame))
			})
		})
	}
}
