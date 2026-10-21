package render

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Item 193: what is drawn again inside the rectangles something moves
// in is played as one stream, a command at a time into every rectangle
// it meets, so the same draw into several rectangles is one run of
// draws rather than one per rectangle, and each rectangle still gets
// its draws in the frame's order.
func TestDirtyStream(t *testing.T) {
	left, right := image.Rect(0, 0, 10, 10), image.Rect(20, 0, 30, 10)
	rects := []image.Rectangle{left, right}
	both := image.Rect(0, 0, 30, 10)
	for _, c := range []struct {
		name    string
		entries []entry
		cmds    []image.Rectangle
		want    []streamPlay
	}{
		{
			"when two rectangles share a drawable",
			[]entry{{inked: inked{from: 0, to: 2, box: both}}},
			[]image.Rectangle{both, both},
			[]streamPlay{{0, 0}, {0, 1}, {1, 0}, {1, 1}},
		},
		{
			"when drawables meet different rectangles",
			[]entry{{inked: inked{from: 0, to: 1, box: right}}, {inked: inked{from: 1, to: 2, box: left}}},
			[]image.Rectangle{right, left},
			[]streamPlay{{0, 1}, {1, 0}},
		},
		{
			"when one of a drawable's draws misses a rectangle it meets",
			[]entry{{inked: inked{from: 0, to: 2, box: both}}},
			[]image.Rectangle{left, both},
			[]streamPlay{{0, 0}, {1, 0}, {1, 1}},
		},
		{
			"when a drawable meets no rectangle",
			[]entry{{inked: inked{from: 0, to: 1, box: image.Rect(50, 50, 60, 60)}}},
			[]image.Rectangle{image.Rect(50, 50, 60, 60)},
			nil,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Run("it should play each draw into each rectangle it meets, a draw at a time, in the frame's order", func(t *testing.T) {
				assert.Equal(t, c.want, stream(nil, c.entries, func(i int) image.Rectangle { return c.cmds[i] }, rects))
			})
		})
	}
}

// Item 193: draws into one rectangle that do not overlap touch different
// pixels, so their order is free, and draws from one source are put
// together so they go down as one; draws that overlap keep their order.
func TestDirtyBatch(t *testing.T) {
	a, b := image.Rect(0, 0, 4, 4), image.Rect(6, 0, 10, 4)
	for _, c := range []struct {
		name  string
		plays []streamPlay
		boxes []image.Rectangle
		srcs  []int
		want  []streamPlay
	}{
		{
			"when draws from two sources alternate and none overlap",
			[]streamPlay{{0, 0}, {1, 0}, {2, 0}, {3, 0}},
			[]image.Rectangle{a, b, a.Add(image.Pt(0, 5)), b.Add(image.Pt(0, 5))},
			[]int{1, 2, 1, 2},
			[]streamPlay{{0, 0}, {2, 0}, {1, 0}, {3, 0}},
		},
		{
			"when a draw overlaps one from another source before it",
			[]streamPlay{{0, 0}, {1, 0}, {2, 0}},
			[]image.Rectangle{a, a, b},
			[]int{1, 2, 1},
			[]streamPlay{{0, 0}, {2, 0}, {1, 0}},
		},
		{
			"when a draw overlaps the draw it would be moved ahead of",
			[]streamPlay{{0, 0}, {1, 0}, {2, 0}},
			[]image.Rectangle{a, a, a},
			[]int{1, 2, 1},
			[]streamPlay{{0, 0}, {1, 0}, {2, 0}},
		},
		{
			"when overlapping draws are in different rectangles",
			[]streamPlay{{0, 0}, {1, 1}, {2, 0}},
			[]image.Rectangle{a, a, a},
			[]int{1, 2, 1},
			[]streamPlay{{0, 0}, {2, 0}, {1, 1}},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := batch(nil, c.plays, func(i int) image.Rectangle { return c.boxes[i] }, func(i int) int { return c.srcs[i] })

			t.Run("it should put draws from one source together without passing a draw they overlap", func(t *testing.T) {
				assert.Equal(t, c.want, got)
			})
		})
	}
}
