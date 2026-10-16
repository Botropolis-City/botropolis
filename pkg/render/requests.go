package render

import (
	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/single"
)

// windowSeam is what the city can ask of its own window. Raise brings it
// to the front where the desktop allows it; item 84 fills that in, and
// until a platform has it the window asks for attention. Item 82.
type windowSeam interface {
	Minimized() bool
	Restore()
	Raise() bool
	RequestAttention()
}

// ebitenWindow is the window Ebitengine gives every desktop: it can be
// restored and can ask for attention, and has no call that raises it.
type ebitenWindow struct{}

func (ebitenWindow) Minimized() bool   { return ebiten.IsWindowMinimized() }
func (ebitenWindow) Restore()          { ebiten.RestoreWindow() }
func (ebitenWindow) Raise() bool       { return false }
func (ebitenWindow) RequestAttention() { ebiten.RequestAttention() }

// answerRequests takes what other commands asked of this, the one city:
// to come forward, or to open the finder and come forward.
func (g *Game) answerRequests() {
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
			g.comeForward()
		default:
			return
		}
	}
}

// comeForward brings the window back from the dock and to the front, or
// asks for attention where the desktop will not raise it.
func (g *Game) comeForward() {
	w := g.win
	if w == nil {
		w = ebitenWindow{}
	}
	if w.Minimized() {
		w.Restore()
	}
	if !w.Raise() {
		w.RequestAttention()
	}
}
