package render

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// A map label's plate and words change far less often than they are
// drawn, and laying words out costs six indices a glyph; so each label
// is recorded, hashed relative to where it lands, baked once it has held
// for a frame, and blitted after (item 193). The labels are baked onto
// one page of their own, so every label's blit reads one texture and
// goes down as one draw; on Ebitengine's shared atlas each new image was
// moved onto it some frames later, a copy and two target switches in a
// frame that changed nothing.

// labelCacheCap is the bytes of label images kept: labels are small, and
// a city shows a few dozen.
const labelCacheCap = 4 << 20

// labelPhases is how many steps a pixel is split into for where a label
// falls inside it: a label baked at one phase is not blitted at another.
const labelPhases = 256

// labelKey is what a label paints, relative to where it lands, and where
// it lands inside a pixel.
type labelKey struct {
	hash  uint64
	phase [2]int16
}

// bakedLabel is a label's image, part of the label page, and where it
// stands from the pixel the label lands in. The page's room comes back
// only when the page fills and starts again, so dropping one frees
// nothing.
type bakedLabel struct {
	img *ebiten.Image
	at  image.Point
}

func (b *bakedLabel) Deallocate() {}

// labelPageSize is the label page's width and height.
const labelPageSize = 1024

// labelPage places labels on one image in shelves, left to right, a new
// shelf under the tallest of the last when a row fills, with a pixel
// between, so linear filtering never reads a neighbour.
type labelPage struct {
	img         *ebiten.Image
	w, h        int
	x, y, shelf int
}

// place is where a label w×h goes on the page, or false when the page
// is full.
func (p *labelPage) place(w, h int) (image.Rectangle, bool) {
	if p.x+w > p.w {
		p.x, p.y, p.shelf = 0, p.y+p.shelf+1, 0
	}
	if p.x+w > p.w || p.y+h > p.h {
		return image.Rectangle{}, false
	}
	r := image.Rect(p.x, p.y, p.x+w, p.y+h)
	p.x += w + 1
	p.shelf = max(p.shelf, h)
	return r, true
}

// reset empties the page.
func (p *labelPage) reset() {
	p.x, p.y, p.shelf = 0, 0, 0
	if p.img != nil {
		p.img.Clear()
	}
}

type labelUse int

const (
	labelStraight labelUse = iota
	labelBake
	labelBaked
)

// labelCache keeps baked labels, least recently used out first, and
// which keys were drawn last frame.
type labelCache struct {
	looks *lookCache[labelKey, *bakedLabel]
	seen  map[labelKey]int
	frame int
	page  labelPage
}

func newLabelCache() *labelCache {
	return &labelCache{looks: newLookCache[labelKey, *bakedLabel](labelCacheCap), seen: map[labelKey]int{}, page: labelPage{w: labelPageSize, h: labelPageSize}}
}

// next starts a frame, forgetting keys not drawn for a while.
func (c *labelCache) next() {
	c.frame++
	if c.frame%lookForget == 0 {
		for k, f := range c.seen {
			if f < c.frame-lookForget {
				delete(c.seen, k)
			}
		}
	}
}

// decide is how a label keyed so is drawn this frame: from its image,
// baked now because it was drawn under the same key last frame, or
// straight.
func (c *labelCache) decide(k labelKey) labelUse {
	last, seen := c.seen[k]
	c.seen[k] = c.frame
	if _, ok := c.looks.get(k); ok {
		return labelBaked
	}
	if seen && last == c.frame-1 {
		return labelBake
	}
	return labelStraight
}

// keep keeps a baked label.
func (c *labelCache) keep(k labelKey, b *bakedLabel, bytes int) {
	c.looks.put(k, b, bytes)
}

// labelPhase is where a point falls inside its pixel, in labelPhases.
func labelPhase(p city.Point) (image.Point, [2]int16) {
	q := func(v float64) (int, int16) {
		steps := int(math.Round(v * labelPhases))
		cell := int(math.Floor(float64(steps) / labelPhases))
		return cell, int16(steps - cell*labelPhases)
	}
	x, px := q(p.X)
	y, py := q(p.Y)
	return image.Pt(x, y), [2]int16{px, py}
}

// drawLabel draws one laid-out label onto dst through the cache.
func (g *Game) drawLabel(dst *ebiten.Image, draw func(*ebiten.Image, city.Rect), at city.Rect) {
	if g.noStill || g.labelLooks == nil {
		draw(dst, at)
		return
	}
	l := g.recorder()
	origin := l.origin
	l.origin = at.Min
	l.begin()
	draw(l.dry, at)
	in := l.end()
	l.origin = origin
	cell, phase := labelPhase(at.Min)
	key := labelKey{hash: in.hash, phase: phase}
	switch g.labelLooks.decide(key) {
	case labelBaked:
		b, _ := g.labelLooks.looks.get(key)
		g.blitLabel(dst, b, cell)
		return
	case labelBake:
		box := in.box
		if box.Empty() {
			return
		}
		lc := g.labelLooks
		r, ok := lc.page.place(box.Dx(), box.Dy())
		if !ok {
			// Full: start the page again, and every label with it.
			lc.page.reset()
			lc.looks = newLookCache[labelKey, *bakedLabel](labelCacheCap)
			r, ok = lc.page.place(box.Dx(), box.Dy())
		}
		if !ok {
			break
		}
		if lc.page.img == nil {
			lc.page.img = sizedImage(nil, image.Pt(lc.page.w, lc.page.h))
		}
		sub := lc.page.img.SubImage(r).(*ebiten.Image)
		l.playShifted(sub, in, r.Min.Sub(box.Min))
		b := &bakedLabel{img: sub, at: box.Min.Sub(cell)}
		lc.keep(key, b, box.Dx()*box.Dy()*4)
		g.blitLabel(dst, b, cell)
		return
	}
	l.play(dst, in, nil)
}

func (g *Game) blitLabel(dst *ebiten.Image, b *bakedLabel, cell image.Point) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(cell.X+b.at.X), float64(cell.Y+b.at.Y))
	dst.DrawImage(b.img, op)
}
