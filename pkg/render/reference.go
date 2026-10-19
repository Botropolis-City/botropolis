package render

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// reference is the reference behind ? (item 88), a tenant of the
// overlay stack: an entry per kind of thing on the map, whose terms
// open their own entries. Its Keys entry is the key list, whose rows
// still press their key when clicked.
// credit thanks the people whose work the city is drawn with, in the
// words the website and the README use, at the foot of the reference's
// index. Item 122.
const credit = "Roads, trees, vehicles, trains, boats and landmarks by Kenney (kenney.nl), CC0 — thank you. " +
	"Session buildings, the drone, the fountain and the building sites are modelled for Botropolis. " +
	"Set in Inter by The Inter Project Authors (SIL OFL 1.1)."

type reference struct {
	ref     *ui.Reference
	laid    ui.ReferencePanel
	keyList ui.Help
	// skip is how many of the key list's rows are scrolled past, and
	// notch the part of a wheel notch not yet scrolled (item 127).
	skip  int
	notch float64
}

func (g *Game) referenceOpen() *reference {
	r, _ := g.overlays.top().(*reference)
	return r
}

func (g *Game) openReference() {
	if g.referenceOpen() != nil {
		return
	}
	g.overlays.push(&reference{ref: ui.NewReference(g.helpKeys(), credit)})
}

func (r *reference) keys(g *Game) bool {
	switch {
	case g.just(ebiten.KeyEscape), (g.just(ebiten.KeySlash) && g.shifted()), g.just(ebiten.KeyF1):
		return false
	case g.just(ebiten.KeyBackspace):
		r.ref.Back()
	case g.just(ebiten.KeyArrowDown):
		r.scroll(g, 1)
	case g.just(ebiten.KeyArrowUp):
		r.scroll(g, -1)
	}
	return true
}

// scroll moves the key list by rows; the layout keeps it in range.
func (r *reference) scroll(g *Game, rows int) {
	if r.ref.Current().ID != ui.KeysEntry {
		return
	}
	g.mu.Lock()
	r.skip = min(max(r.skip+rows, 0), r.keyList.Max)
	g.mu.Unlock()
}

// wheel scrolls the key list by whole rows, and takes the wheel whatever
// entry is open, so the map behind the reference does not zoom.
func (r *reference) wheel(g *Game, _ city.Point, notches float64) bool {
	r.notch += notches
	whole := math.Trunc(r.notch)
	r.notch -= whole
	r.scroll(g, -int(whole))
	return true
}

func (r *reference) click(g *Game, at city.Point) bool {
	g.mu.Lock()
	laid, keys := r.laid, r.keyList
	g.mu.Unlock()
	if r.ref.Current().ID == ui.KeysEntry {
		if hit, ok := keys.Hit(at); ok {
			if key, ok := press(hit.Key); ok {
				g.clicked = key
			}
			return false
		}
		return keys.Rect.Contains(at)
	}
	if term, ok := laid.Hit(at); ok {
		r.ref.Open(term)
		return true
	}
	return laid.Rect.Contains(at)
}

func (r *reference) draw(g *Game, screen *ebiten.Image, width, height float64) {
	th := g.theme
	vector.FillRect(screen, 0, 0, float32(width), float32(height), colorScrim, false)
	if r.ref.Current().ID == ui.KeysEntry {
		g.mu.Lock()
		skip := r.skip
		top, foot := 0.0, height
		var covers []city.Rect
		if !g.hidden && g.footerBar.Rect.Area() > 0 {
			top, foot = g.scene.TopChrome(), min(g.footerBar.Rect.Min.Y, height)
			for _, c := range []city.Rect{g.streamLaid.Rect, g.minimapBox} {
				if c.Area() > 0 {
					covers = append(covers, c)
				}
			}
		}
		g.mu.Unlock()
		room := city.Rect{Min: city.Point{Y: top}, Max: city.Point{X: width, Y: foot}}
		h := ui.LayoutHelpIn(th, room, covers, "Keys", g.helpKeys(), g.faces.Measure, skip)
		g.mu.Lock()
		r.keyList = h
		r.skip = h.Skip
		g.mu.Unlock()
		g.roundPanel(screen, h.Rect)
		g.run(screen, h.Title, th.Palette.Text)
		for _, k := range h.Rows {
			g.roundRect(screen, k.Chip, th.Radius()/2, th.Palette.Hairline)
			for _, l := range k.KeyLines {
				g.run(screen, l, th.Palette.Text)
			}
			for _, l := range k.Label {
				g.run(screen, l, th.Palette.Dim)
			}
		}
		g.run(screen, h.Above, th.Palette.Dim)
		g.run(screen, h.Below, th.Palette.Dim)
		return
	}
	p := ui.LayoutReference(th, width, height, r.ref, g.faces.Measure)
	g.mu.Lock()
	r.laid, r.skip = p, 0
	g.mu.Unlock()
	g.roundPanel(screen, p.Rect)
	g.run(screen, p.Title, th.Palette.Text)
	g.run(screen, p.Subtitle, th.Palette.Dim)
	for _, l := range p.Lines {
		g.run(screen, l, th.Palette.Text)
	}
	for _, b := range p.Terms {
		g.roundRect(screen, b.Rect, th.Radius()/2, th.Palette.Hairline)
		g.run(screen, b.Label, th.Palette.Accent)
	}
	g.run(screen, p.Hint, th.Palette.Dim)
	for _, l := range p.Credit {
		g.run(screen, l, th.Palette.Dim)
	}
}
