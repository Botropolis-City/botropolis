package render

import (
	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/single"
)

// windowSeam is what the city can ask of its own window. Raise brings it
// to the front where the desktop allows it, with the time of the press
// that asked, or 0; where a platform cannot, the window asks for
// attention. Items 82 and 84.
type windowSeam interface {
	Minimized() bool
	Restore()
	Raise(time uint32) bool
	RequestAttention()
}

// shortcutSeam is the global shortcut, where the desktop has one: the
// combination grabbed, and the times it was pressed at. Item 84.
type shortcutSeam interface {
	Grab(c config.Chord) error
	Release() error
	Fired() <-chan uint32
}

// ebitenWindow is the window Ebitengine gives every desktop: it can be
// restored and can ask for attention, and has no call that raises it.
type ebitenWindow struct{}

func (ebitenWindow) Minimized() bool   { return ebiten.IsWindowMinimized() }
func (ebitenWindow) Restore()          { ebiten.RestoreWindow() }
func (ebitenWindow) Raise(uint32) bool { return false }
func (ebitenWindow) RequestAttention() { ebiten.RequestAttention() }

// raisedWindow is Ebitengine's window raised through the desktop, where
// the desktop has a way to. Item 84.
type raisedWindow struct {
	ebitenWindow
	raise func(time uint32) bool
}

func (w raisedWindow) Raise(time uint32) bool { return w.raise(time) }

// answerRequests takes what other commands asked of this, the one city:
// to come forward, or to open the finder and come forward; and the
// global shortcut, which asks the same as botropolis find.
func (g *Game) answerRequests() {
	var fired <-chan uint32
	if g.shortcut != nil {
		fired = g.shortcut.Fired()
	}
	for {
		select {
		case r := <-g.requests:
			if r.Op == single.Find {
				g.openFinder()
				if f, ok := g.overlays.top().(*finder); ok && r.Words != "" {
					f.field = field{}
					f.field.insert(r.Words)
				}
			}
			g.comeForward(0)
		case at := <-fired:
			g.openFinder()
			g.comeForward(at)
		default:
			return
		}
	}
}

// comeForward brings the window back from the dock and to the front, or
// asks for attention where the desktop will not raise it.
func (g *Game) comeForward(time uint32) {
	w := g.win
	if w == nil {
		w = ebitenWindow{}
	}
	if w.Minimized() {
		w.Restore()
	}
	if !w.Raise(time) {
		w.RequestAttention()
	}
}
