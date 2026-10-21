package render

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Item 193: a map label is drawn from an image baked once it has held
// for a frame, rather than laid out glyph by glyph every frame.
func TestLabelCache(t *testing.T) {
	// frames runs a label cache over frames, each drawing the labels
	// keyed so, and counts the bakes.
	frames := func(frames ...[]labelKey) (bakes int, last []labelUse) {
		c := newLabelCache()
		for _, keys := range frames {
			c.next()
			last = last[:0]
			for _, k := range keys {
				use := c.decide(k)
				if use == labelBake {
					bakes++
					c.keep(k, &bakedLabel{}, 64)
				}
				last = append(last, use)
			}
		}
		return bakes, last
	}
	label := labelKey{hash: 1}

	t.Run("when a label is drawn three frames unchanged", func(t *testing.T) {
		bakes, last := frames([]labelKey{label}, []labelKey{label}, []labelKey{label})

		t.Run("it should render its words once", func(t *testing.T) {
			assert.Equal(t, 1, bakes)
		})

		t.Run("it should draw it from the baked image after", func(t *testing.T) {
			assert.Equal(t, []labelUse{labelBaked}, last)
		})
	})

	t.Run("when labels are placed on a page of their own", func(t *testing.T) {
		p := &shelfPage{w: 100, h: 40}
		a, _ := p.place(60, 10)
		b, _ := p.place(30, 12)
		c, _ := p.place(50, 8)
		_, full := p.place(90, 30)

		t.Run("it should set them side by side while they fit", func(t *testing.T) {
			assert.Equal(t, []image.Rectangle{image.Rect(0, 0, 60, 10), image.Rect(61, 0, 91, 12)}, []image.Rectangle{a, b})
		})

		t.Run("it should start a shelf under the tallest when the row is full", func(t *testing.T) {
			assert.Equal(t, image.Rect(0, 13, 50, 21), c)
		})

		t.Run("it should say when the page is full", func(t *testing.T) {
			assert.False(t, full)
		})
	})

	t.Run("when a label is drawn for one frame only", func(t *testing.T) {
		bakes, _ := frames([]labelKey{label})

		t.Run("it should bake nothing", func(t *testing.T) {
			assert.Zero(t, bakes)
		})
	})

	for _, c := range []struct {
		name  string
		other labelKey
	}{
		{"its words, size or fade", labelKey{hash: 2}},
		{"where it falls inside a pixel", labelKey{hash: 1, phase: [2]int16{3, 0}}},
	} {
		t.Run("when "+c.name+" change after it was baked", func(t *testing.T) {
			bakes, _ := frames([]labelKey{label}, []labelKey{label}, []labelKey{c.other}, []labelKey{c.other})

			t.Run("it should bake it again", func(t *testing.T) {
				assert.Equal(t, 2, bakes)
			})
		})
	}
}
