package render

import (
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
	}
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
		h := ui.LayoutHelp(th, width, height, "Keys", g.helpKeys(), g.faces.Measure)
		g.mu.Lock()
		r.keyList = h
		g.mu.Unlock()
		g.roundPanel(screen, h.Rect)
		g.run(screen, h.Title, th.Palette.Text)
		g.keyRow(screen, h.Rows, ui.Small)
		return
	}
	p := ui.LayoutReference(th, width, height, r.ref, g.faces.Measure)
	g.mu.Lock()
	r.laid = p
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
