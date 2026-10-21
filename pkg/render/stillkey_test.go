package render

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Item 193: the still layer is composed again exactly where something
// still would paint differently, and never because something moved.
func TestStillLayer(t *testing.T) {
	box := func(x int) image.Rectangle { return image.Rect(x, 0, x+10, 10) }
	still := func(hash uint64, x int) entry {
		return entry{inked: inked{hash: hash, box: box(x)}}
	}
	mover := func(hash uint64, x int) entry {
		e := still(hash, x)
		e.moves = true
		return e
	}
	composed := newStillSet([]entry{still(1, 0), mover(2, 20), still(3, 40)})

	for _, c := range []struct {
		name  string
		frame []entry
		want  []image.Rectangle
	}{
		{"when nothing changes between frames", []entry{still(1, 0), mover(2, 20), still(3, 40)}, nil},
		{"when only a mover moves", []entry{still(1, 0), mover(9, 25), still(3, 40)}, nil},
		{"when a mover leaves", []entry{still(1, 0), still(3, 40)}, nil},
		{"when two stills that cannot meet swap places in the order", []entry{still(3, 40), mover(2, 20), still(1, 0)}, nil},
		{"when a still's draws change", []entry{still(1, 0), mover(2, 20), still(4, 40)}, []image.Rectangle{box(40)}},
		{"when a still moves to a new box", []entry{still(1, 0), mover(2, 20), still(3, 60)}, []image.Rectangle{box(40), box(60)}},
		{"when a still goes", []entry{still(1, 0), mover(2, 20)}, []image.Rectangle{box(40)}},
		{"when a still starts to move", []entry{still(1, 0), mover(2, 20), mover(3, 40)}, []image.Rectangle{box(40)}},
		{"when a still is drawn twice", []entry{still(1, 0), still(1, 0), mover(2, 20), still(3, 40)}, []image.Rectangle{box(0)}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Run("it should compose again inside exactly the boxes that changed", func(t *testing.T) {
				assert.ElementsMatch(t, c.want, composed.changes(nil, c.frame))
			})
		})
	}

	t.Run("when the set is moved on to a frame", func(t *testing.T) {
		set := newStillSet([]entry{still(1, 0)})
		next := []entry{still(1, 0), still(5, 30)}
		set.reset(next)

		t.Run("it should find nothing changed in that frame", func(t *testing.T) {
			assert.Empty(t, set.changes(nil, next))
		})
	})
}
