package render

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// A panel's words changed far less often than they were drawn, and on
// the real history the journal and the group review drew every glyph
// of theirs every frame: most of the text the city drew at fit (item
// 161). A panel that opts in is drawn into the recorder instead: its
// code runs every frame, so what it notes as it draws still happens,
// and the record's hash says whether it would paint what it painted the
// frame before. New this frame, the record is played straight; held for
// a frame, it is baked into an image the panel's size; held since, the
// image is blitted. The map labels are baked the same way.

// bakesPanel is an overlay whose every draw goes through the recorder,
// so it can be recorded, baked and blitted while it holds. A panel that
// draws anything itself, as the reader's runs do, must not opt in: the
// draw would land on the recorder's stand-in and be lost.
type bakesPanel interface {
	bakes()
}

func (*journal) bakes()  {}
func (*firstRun) bakes() {}

// bakeKey names a region of the chrome that is baked while it holds, as
// the panels are (item 161). The footer is not one: its words went down
// in one draw with the text drawn after them, and a blit of its own made
// the night's frame a draw command more.
type bakeKey int

const (
	bakeStrip bakeKey = iota + 1
	bakeStream
	bakeTimeline
)

// panelBakes are the baked panels and regions, by the overlay or the
// region drawn, and what they cost this frame.
type panelBakes struct {
	held  map[any]*panelBake
	stats panelStats
	// frame counts the frames the bakes were drawn through.
	frame int
	// straight draws every panel straight, as the check against the
	// bakes does.
	straight bool
}

// panelBake is one bake: the image, the box it covers on the frame, and
// the hash of the record it was baked from; held is the hash the last
// frame recorded. seen is the frame it was last drawn through, and
// drawnAt the last frame its words were drawn, straight or baked.
type panelBake struct {
	img           *ebiten.Image
	box           image.Rectangle
	hash, held    uint64
	ok            bool
	seen, drawnAt int
}

// panelStats counts, for a frame, the panels drawn straight, baked and
// blitted.
type panelStats struct {
	straight, bakes, blits int
}

// drawPanel draws an overlay, through its bake when it opts in.
func (g *Game) drawPanel(o overlay, screen *ebiten.Image, width, height float64) {
	if _, ok := o.(bakesPanel); !ok {
		o.draw(g, screen, width, height)
		return
	}
	g.baked(o, screen, func(dst *ebiten.Image) { o.draw(g, dst, width, height) })
}

// baked runs draw, which must draw only through the recorder, onto the
// recorder's stand-in, and puts what it drew on screen through the bake
// kept under key: its code runs every frame, so what it notes as it
// draws still happens, and only its draws are held.
func (g *Game) baked(key any, screen *ebiten.Image, draw func(dst *ebiten.Image)) {
	if g.panels.straight {
		draw(screen)
		return
	}
	l := g.recorder()
	m := l.mark()
	defer l.truncate(m)
	// What is baked stands still on the screen, so its draws are hashed
	// where they land, not relative to the camera.
	origin := l.origin
	l.origin = city.Point{}
	l.begin()
	draw(l.dry)
	in := l.end()
	l.origin = origin

	if g.panels.held == nil {
		g.panels.held = map[any]*panelBake{}
	}
	p := g.panels.held[key]
	if p == nil {
		p = &panelBake{}
		g.panels.held[key] = p
	}
	p.seen = g.panels.frame
	// The scrim a panel lays over the city first covers the whole frame,
	// and baked with it each bake was a frame-sized texture: 16 MiB at
	// 1920×1200, rounded to a power of two. It is one draw, so it is
	// drawn each frame and only what lies on it is baked.
	frame := screen.Bounds()
	under, on := in, in
	for under.to = in.from; under.to < in.to && l.cmdBox(under.to).Intersect(frame) == frame; under.to++ {
	}
	on.from, on.box = under.to, image.Rectangle{}
	for i := on.from; i < on.to; i++ {
		on.box = on.box.Union(l.cmdBox(i))
	}
	box := on.box.Intersect(frame)
	held := p.held == in.hash
	p.held = in.hash
	if !held || box.Empty() {
		p.ok = false
		l.play(screen, in, nil)
		g.panels.stats.straight++
		p.drawnAt = g.panels.frame
		return
	}
	l.play(screen, under, nil)
	if !p.ok || p.hash != in.hash || p.box != box {
		p.img = sizedImage(p.img, box.Size())
		p.img.Clear()
		l.playShifted(p.img, on, box.Min.Mul(-1))
		p.hash, p.box, p.ok = in.hash, box, true
		g.panels.stats.bakes++
		p.drawnAt = g.panels.frame
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(box.Min.X), float64(box.Min.Y))
	screen.DrawImage(p.img, op)
	g.panels.stats.blits++
}

// prunePanels starts a frame of the bakes, and lets go of each not
// drawn through in the frame before: a panel closed, a region not shown.
func (g *Game) prunePanels() {
	g.panels.frame++
	for k, p := range g.panels.held {
		if p.seen < g.panels.frame-1 {
			if p.img != nil {
				p.img.Deallocate()
			}
			delete(g.panels.held, k)
		}
	}
}
