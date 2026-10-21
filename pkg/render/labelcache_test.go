package render

import (
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
