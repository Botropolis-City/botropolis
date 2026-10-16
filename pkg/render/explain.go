package render

import (
	"slices"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// seen is every kind of thing the last frame drew, where it drew it.
func (g *Game) seen() []ui.Seen {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]ui.Seen, 0, len(g.hits))
	for _, h := range g.hits {
		if k := city.KindOf(h.hit); k != city.KindNone {
			out = append(out, ui.Seen{Kind: k, Rect: h.rect})
		}
	}
	return out
}

// explainHeld is whether the explain key is held, by hand or by a
// script (item 88), over the map: in a panel that takes the keyboard,
// such as the finder, it is a letter being typed.
func (g *Game) explainHeld() bool {
	key, ok := keyByName(g.findChords().Explain.Key)
	if !ok || g.overlays.top() != nil {
		return false
	}
	return g.scriptedHeld[key] || ebiten.IsKeyPressed(key)
}

// drawExplain is explain mode while its key is held: a caption on one
// of each kind on screen. Otherwise it is the first sightings: a kind
// that appears for the first time on this machine says what it is,
// once.
func (g *Game) drawExplain(screen *ebiten.Image, width, height float64) {
	seen := g.seen()
	kinds := make([]city.Kind, 0, len(seen))
	for _, s := range seen {
		if !slices.Contains(kinds, s.Kind) {
			kinds = append(kinds, s.Kind)
		}
	}
	g.scene.Sight(kinds)
	held := g.explainHeld()
	if !held {
		up := g.scene.Sightings()
		seen = slices.DeleteFunc(seen, func(s ui.Seen) bool { return !slices.Contains(up, s.Kind) })
	}
	th := g.theme
	top := g.scene.TopChrome()
	window := city.RectAt(0, top, width, height-top-g.scene.Insets().Bottom)
	g.captions = ui.LayoutExplain(th, seen, window, held || len(seen) > 0, g.faces.Measure)
	for _, c := range g.captions {
		g.roundRect(screen, c.Plate.Rect, th.Radius()/2, th.Palette.Panel)
		g.text(screen, c.Plate.TextAt, c.Text, c.Plate.Size, th.Palette.Text)
	}
}
