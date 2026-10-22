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

// panelBakes are the baked panels, by the overlay drawn, and what they
// cost this frame.
type panelBakes struct {
	held  map[overlay]*panelBake
	stats panelStats
	// straight draws every panel straight, as the check against the
	// bakes does.
	straight bool
}

// panelBake is one panel's bake: the image, the box it covers on the
// frame, and the hash of the record it was baked from; held is the hash
// the last frame recorded.
type panelBake struct {
	img        *ebiten.Image
	box        image.Rectangle
	hash, held uint64
	ok         bool
}

// panelStats counts, for a frame, the panels drawn straight, baked and
// blitted.
type panelStats struct {
	straight, bakes, blits int
}

// drawPanel draws an overlay, through its bake when it opts in.
func (g *Game) drawPanel(o overlay, screen *ebiten.Image, width, height float64) {
	if _, ok := o.(bakesPanel); !ok || g.panels.straight {
		o.draw(g, screen, width, height)
		return
	}
	l := g.recorder()
	m := l.mark()
	defer l.truncate(m)
	// A panel stands still on the screen, so its draws are hashed where
	// they land, not relative to the camera.
	origin := l.origin
	l.origin = city.Point{}
	l.begin()
	o.draw(g, l.dry, width, height)
	in := l.end()
	l.origin = origin

	if g.panels.held == nil {
		g.panels.held = map[overlay]*panelBake{}
	}
	p := g.panels.held[o]
	if p == nil {
		p = &panelBake{}
		g.panels.held[o] = p
	}
	box := in.box.Intersect(screen.Bounds())
	if box.Empty() {
		return
	}
	held := p.held == in.hash
	p.held = in.hash
	if !p.ok || p.hash != in.hash || p.box != box {
		if !held {
			p.ok = false
			l.play(screen, in, nil)
			g.panels.stats.straight++
			return
		}
		p.img = sizedImage(p.img, box.Size())
		p.img.Clear()
		l.playShifted(p.img, in, box.Min.Mul(-1))
		p.hash, p.box, p.ok = in.hash, box, true
		g.panels.stats.bakes++
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(box.Min.X), float64(box.Min.Y))
	screen.DrawImage(p.img, op)
	g.panels.stats.blits++
}

func (s overlays) has(o overlay) bool {
	for _, x := range s {
		if x == o {
			return true
		}
	}
	return false
}

// prunePanels lets go of the bakes of panels no longer open.
func (g *Game) prunePanels() {
	for o, p := range g.panels.held {
		if !g.overlays.has(o) {
			if p.img != nil {
				p.img.Deallocate()
			}
			delete(g.panels.held, o)
		}
	}
}
