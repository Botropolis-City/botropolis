package render

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// loadingEvery is how often a city waiting on the daemon's first load
// asks how far it has got (item 159).
const loadingEvery = 250 * time.Millisecond

// sweepEvery is how long the sweeping bar takes to cross, when nothing
// measures the wait.
const sweepEvery = 1500 * time.Millisecond

// openingWait is what the city is waiting on before it can draw the
// city: its atlas pages, and the first snapshot.
type openingWait struct {
	pagesUp, pages int
	opened         bool
	load           state.LoadProgress
	waited         time.Duration
}

// openingFor is the opening panel's line and fill for a wait, and
// whether to show it at all: not once the city has opened, and not for
// a wait too short to read.
func openingFor(w openingWait) (string, float64, bool) {
	if !ui.OpeningShown(w.waited) {
		return "", 0, false
	}
	if w.pagesUp < w.pages {
		line, fill := ui.OpeningPages(w.pagesUp, w.pages)
		return line, fill, true
	}
	if w.opened {
		return "", 0, false
	}
	line, fill := ui.OpeningLoad(w.load)
	return line, fill, true
}

// drawOpening draws the void, with the opening panel on it once the wait
// is long enough to show, while pagesUp of pages are up and until the
// city opens. It reports whether it drew the frame, which it does not
// once there is nothing left to wait for.
func (g *Game) drawOpening(screen *ebiten.Image, pagesUp, pages int) bool {
	opened := g.scene.Opened()
	g.opened.Store(opened)
	if opened && !g.openLogged {
		g.openLogged = true
		g.logOpened()
	}
	if pagesUp >= pages && opened {
		return false
	}
	screen.Fill(colorVoid)
	w := openingWait{pagesUp: pagesUp, pages: pages, opened: opened, waited: time.Since(g.started)}
	if load := g.loading.Load(); load != nil {
		w.load = *load
	}
	line, fill, shown := openingFor(w)
	if !shown {
		return true
	}
	th := g.theme
	bounds := screen.Bounds()
	o := ui.LayoutOpening(th, float64(bounds.Dx()), float64(bounds.Dy()), line, fill, g.faces.Measure)
	o = o.Sweep(float64(w.waited%sweepEvery) / float64(sweepEvery))
	g.roundPanel(screen, o.Panel)
	g.run(screen, o.Line, th.Palette.Dim)
	g.roundRect(screen, o.Bar, o.Bar.Height()/2, th.Palette.Hairline)
	if o.Fill.Width() > 0 {
		g.roundRect(screen, o.Fill, o.Fill.Height()/2, th.Palette.Accent)
	}
	g.logOpening(line)
	g.keepOpening(screen)
	return true
}

// watchLoading asks how far the first load has got every interval and
// keeps each answer, until the city has opened.
func watchLoading(every time.Duration, opened func() bool, ask func() (state.LoadProgress, error), keep func(*state.LoadProgress)) {
	for !opened() {
		if load, err := ask(); err == nil {
			keep(&load)
		}
		time.Sleep(every)
	}
}
