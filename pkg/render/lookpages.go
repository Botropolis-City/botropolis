package render

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

// Building looks are baked onto pages of their own rather than into
// images of their own (item 193). Ebitengine keeps a small image on its
// shared atlas and moves it off to draw into it, then back some frames
// after it was last drawn into: a copy and two render-target switches,
// in a frame where nothing changed, for every look baked. A page is
// drawn into only when a look is baked on it, and never moved.

// lookPageSize is a look page's width and height: lookCacheCap holds
// lookCacheCap / (lookPageSize² × 4) of them.
const lookPageSize = 1024

// lookPages places looks on pages in shelves, a page opened as the last
// fills, up to a number of them; when all are full, the oldest starts
// again at the next frame, its looks forgotten.
type lookPages struct {
	size   int
	most   int
	pages  []shelfPage
	keys   [][]lookKey
	oldest int
	// full is whether a look found no room this frame.
	full bool
}

func newLookPages(most, size int) *lookPages {
	return &lookPages{most: most, size: size}
}

// place is a look's room on a page, its page and whether it found one.
// A look bigger than a page finds none, and the pages are not full for
// it.
func (p *lookPages) place(size image.Point, key lookKey) (image.Rectangle, int, bool) {
	if size.X > p.size || size.Y > p.size {
		return image.Rectangle{}, 0, false
	}
	for i := range p.pages {
		if r, ok := p.pages[i].place(size.X, size.Y); ok {
			p.keys[i] = append(p.keys[i], key)
			return r, i, true
		}
	}
	if len(p.pages) < p.most {
		p.pages = append(p.pages, shelfPage{w: p.size, h: p.size})
		p.keys = append(p.keys, nil)
		i := len(p.pages) - 1
		r, _ := p.pages[i].place(size.X, size.Y)
		p.keys[i] = append(p.keys[i], key)
		return r, i, true
	}
	p.full = true
	return image.Rectangle{}, 0, false
}

// next starts a frame: when the pages were full, the oldest starts
// again, and forget is told of every look that was on it.
func (p *lookPages) next(forget func(lookKey)) {
	if !p.full || len(p.pages) == 0 {
		return
	}
	p.full = false
	i := p.oldest
	for _, k := range p.keys[i] {
		forget(k)
	}
	p.keys[i] = p.keys[i][:0]
	p.pages[i].reset()
	p.oldest = (i + 1) % len(p.pages)
}

// image is page i's image, made the first time it is drawn into.
func (p *lookPages) image(i int) *ebiten.Image {
	if p.pages[i].img == nil {
		p.pages[i].img = sizedImage(nil, image.Pt(p.size, p.size))
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
