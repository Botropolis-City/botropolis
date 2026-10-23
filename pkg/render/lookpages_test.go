package render

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
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
		p.next(2, func(k lookKey) { forgot = append(forgot, k) })
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

// Item 162: looks are baked at full zoom and not at fit, so the pages a
// zoom in made were kept for the life of the city: 24 MiB more at fit
// after zooming in once and back.
func TestLookPagesLetGo(t *testing.T) {
	baked := func() *Game {
		g := &Game{}
		g.nextLookFrame()
		g.lookPages = newLookPages(2, 64)
		room, page, _ := g.lookPages.place(image.Pt(16, 16), lookKey{id: "a"})
		look := &bakedLook{img: g.lookPages.image(page).SubImage(room).(*ebiten.Image), paged: true}
		g.looks.put(lookKey{id: "a"}, look, 16*16*4)
		g.lookStats.bakes++
		return g
	}
	images := func(g *Game) int {
		n := 0
		if g.lookPages != nil {
			for _, p := range g.lookPages.pages {
				if p.img != nil {
					n++
				}
			}
		}
		return n
	}

	t.Run("when looks have been baked onto pages", func(t *testing.T) {
		t.Run("and no look is drawn for lookForget frames", func(t *testing.T) {
			g := baked()
			for range lookForget + 1 {
				g.nextLookFrame()
			}

			t.Run("it should hold no look-page image", func(t *testing.T) {
				assert.Zero(t, images(g))
			})
		})

		t.Run("and a look is drawn within lookForget frames", func(t *testing.T) {
			g := baked()
			for i := range lookForget + 1 {
				g.nextLookFrame()
				if i == lookForget/2 {
					g.lookStats.hits++
				}
			}

			t.Run("it should keep its pages", func(t *testing.T) {
				assert.Equal(t, 1, images(g))
			})
		})
	})
}

// Item 161: when every page was full, the oldest started again at the
// next frame even while its looks were on screen, so they were baked
// again and the page filled again, every frame. Only a page none of
// whose looks were drawn the frame before starts again, the one drawn
// longest ago.
func TestLookPagesKeepWhatIsDrawn(t *testing.T) {
	key := func(id string) lookKey { return lookKey{id: id} }
	// full is two pages of one look each, a on page 0 and b on page 1,
	// called full by a third that found no room.
	full := func() *lookPages {
		p := newLookPages(2, 100)
		p.place(image.Pt(90, 90), key("a"))
		p.place(image.Pt(90, 90), key("b"))
		p.place(image.Pt(90, 90), key("c"))
		return p
	}
	next := func(p *lookPages, frame int) []lookKey {
		var forgot []lookKey
		p.next(frame, func(k lookKey) { forgot = append(forgot, k) })
		return forgot
	}

	t.Run("when every page is full", func(t *testing.T) {
		t.Run("and every page held a look drawn the frame before", func(t *testing.T) {
			p := full()
			p.use(0, 10)
			p.use(1, 10)

			t.Run("it should start none of them again", func(t *testing.T) {
				assert.Empty(t, next(p, 11))
			})
		})

		t.Run("and one page held only looks not drawn the frame before", func(t *testing.T) {
			p := full()
			p.use(0, 10)
			p.use(1, 4)

			t.Run("it should start that page again, not the oldest", func(t *testing.T) {
				assert.Equal(t, []lookKey{key("b")}, next(p, 11))
			})
		})
	})
}
