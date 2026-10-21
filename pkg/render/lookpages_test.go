package render

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Item 193: building looks are baked onto pages of their own, so
// Ebitengine never moves them onto its atlas some frames later; a full
// page starts again only at the start of a frame, so no look drawn that
// frame is wiped before it goes down.
func TestLookPages(t *testing.T) {
	key := func(id string) lookKey { return lookKey{id: id} }

	t.Run("when looks fit on the pages", func(t *testing.T) {
		p := newLookPages(2, 100)
		_, a, okA := p.place(image.Pt(60, 60), key("a"))
		_, b, okB := p.place(image.Pt(60, 60), key("b"))

		t.Run("it should place each", func(t *testing.T) {
			assert.Equal(t, []bool{true, true}, []bool{okA, okB})
		})

		t.Run("it should open a second page when the first has no room", func(t *testing.T) {
			assert.Equal(t, []int{0, 1}, []int{a, b})
		})
	})

	t.Run("when every page is full", func(t *testing.T) {
		p := newLookPages(1, 100)
		p.place(image.Pt(90, 90), key("a"))
		_, _, ok := p.place(image.Pt(90, 90), key("b"))
		var forgot []lookKey
		p.next(func(k lookKey) { forgot = append(forgot, k) })
		_, page, again := p.place(image.Pt(90, 90), key("b"))

		t.Run("it should place nothing more that frame", func(t *testing.T) {
			assert.False(t, ok)
		})

		t.Run("it should forget the oldest page's looks at the next frame", func(t *testing.T) {
			assert.Equal(t, []lookKey{key("a")}, forgot)
		})

		t.Run("it should place on that page again after", func(t *testing.T) {
			assert.Equal(t, []any{true, 0}, []any{again, page})
		})
	})

	t.Run("when a look is bigger than a page", func(t *testing.T) {
		p := newLookPages(2, 100)
		_, _, ok := p.place(image.Pt(120, 20), key("a"))

		t.Run("it should not place it, and leave it to an image of its own", func(t *testing.T) {
			assert.False(t, ok)
		})

		t.Run("it should not call the pages full", func(t *testing.T) {
			assert.False(t, p.full)
		})
	})
}
