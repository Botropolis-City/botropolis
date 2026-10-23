package render

import (
	"image"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
)

// Building looks are baked onto pages of their own rather than into
// images of their own (item 193). Ebitengine keeps a small image on its
// shared atlas and moves it off to draw into it, then back some frames
// after it was last drawn into: a copy and two render-target switches,
// in a frame where nothing changed, for every look baked. A page is
// drawn into only when a look is baked on it, and never moved.

// lookPageSizes are the look pages' widths and heights, in the order
// they open, lookCacheCap in all (item 161). Four small pages are
// lookCacheCap's at r905, so a city at fit, whose looks fill a fraction
// of one, holds 4 MiB of them; at full zoom shelves on small pages
// wasted enough that the looks on screen found no room, and the large
// page, opened last, holds them closer.
var lookPageSizes = []int{1024, 1024, 1024, 1024, 2048}

// lookPages places looks on pages in shelves, a page opened as the last
// fills, up to a number of them; when all are full, the one drawn from
// longest ago starts again at the next frame, its looks forgotten.
type lookPages struct {
	sizes []int
	pages []shelfPage
	keys  [][]lookKey
	// used is the frame each page last had a look drawn from (item 161).
	used []int
	// full is whether a look found no room this frame.
	full bool
}

func newLookPages(most, size int) *lookPages {
	sizes := make([]int, most)
	for i := range sizes {
		sizes[i] = size
	}
	return newLookPageSizes(sizes...)
}

// newLookPageSizes is pages of these sizes, opened in this order.
func newLookPageSizes(sizes ...int) *lookPages {
	return &lookPages{sizes: sizes}
}

// place is a look's room on a page, its page and whether it found one.
// A look bigger than the page it would open finds none, and the pages
// are not full for it.
func (p *lookPages) place(size image.Point, key lookKey) (image.Rectangle, int, bool) {
	if side := slices.Max(p.sizes); size.X > side || size.Y > side {
		return image.Rectangle{}, 0, false
	}
	for i := range p.pages {
		if r, ok := p.pages[i].place(size.X, size.Y); ok {
			p.keys[i] = append(p.keys[i], key)
			return r, i, true
		}
	}
	if len(p.pages) < len(p.sizes) {
		side := p.sizes[len(p.pages)]
		if size.X > side || size.Y > side {
			return image.Rectangle{}, 0, false
		}
		p.pages = append(p.pages, shelfPage{w: side, h: side})
		p.keys = append(p.keys, nil)
		p.used = append(p.used, 0)
		i := len(p.pages) - 1
		r, _ := p.pages[i].place(size.X, size.Y)
		p.keys[i] = append(p.keys[i], key)
		return r, i, true
	}
	p.full = true
	return image.Rectangle{}, 0, false
}

// use notes that a look on page i was drawn at frame.
func (p *lookPages) use(i, frame int) { p.used[i] = frame }

// next starts a frame: when the pages were full, the page drawn from
// longest ago starts again, and forget is told of every look that was
// on it; a page drawn from the frame before is on screen, and none of
// those starts again (item 161).
func (p *lookPages) next(frame int, forget func(lookKey)) {
	if !p.full || len(p.pages) == 0 {
		return
	}
	p.full = false
	i := -1
	for j := range p.pages {
		if p.used[j] < frame-1 && (i < 0 || p.used[j] < p.used[i]) {
			i = j
		}
	}
	if i < 0 {
		return
	}
	for _, k := range p.keys[i] {
		forget(k)
	}
	p.keys[i] = p.keys[i][:0]
	p.pages[i].reset()
}

// image is page i's image, made the first time it is drawn into.
func (p *lookPages) image(i int) *ebiten.Image {
	if p.pages[i].img == nil {
		p.pages[i].img = sizedImage(nil, image.Pt(p.pages[i].w, p.pages[i].h))
	}
	return p.pages[i].img
}

// release frees the pages' images; a nil lookPages has none.
func (p *lookPages) release() {
	if p == nil {
		return
	}
	for i := range p.pages {
		if p.pages[i].img != nil {
			p.pages[i].img.Deallocate()
			p.pages[i].img = nil
		}
	}
}
