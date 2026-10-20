package render

import (
	"slices"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// seen is every kind of thing the last frame drew, where it drew it;
// drawing, the frame so far.
func (g *Game) seen(drawing bool) []ui.Seen {
	g.mu.Lock()
	defer g.mu.Unlock()
	hits := g.hits
	if drawing {
		hits = g.frameHits
	}
	out := make([]ui.Seen, 0, len(hits))
	for _, h := range hits {
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

// explainCaptions is explain mode while its key is held: a caption on
// one of each kind on screen. Otherwise it is the first sightings: a
// kind that appears for the first time on this machine says what it
// is, once. Those are labels, so z hides them (item 102); explain mode
// was asked for, and does not.
func (g *Game) explainCaptions(held, drawing bool, width, height float64) []ui.Caption {
	if !held && !g.scene.Labels() {
		return nil
	}
	seen := g.seen(drawing)
	kinds := make([]city.Kind, 0, len(seen))
	for _, s := range seen {
		if !slices.Contains(kinds, s.Kind) {
			kinds = append(kinds, s.Kind)
		}
	}
	g.scene.Sight(kinds)
	if !held {
		up := g.scene.Sightings()
		seen = slices.DeleteFunc(seen, func(s ui.Seen) bool { return !slices.Contains(up, s.Kind) })
	}
	top := g.scene.TopChrome()
	window := city.RectAt(0, top, width, height-top-g.scene.Insets().Bottom)
	return ui.LayoutExplain(g.theme, seen, window, held || len(seen) > 0, g.faces.Measure)
}

// queueSightings lays the first sightings' captions out with the map's
// other labels, so one cannot land on a sign or a badge (item 102). The
// map is isometric here; drawExplain draws them in the other views.
func (g *Game) queueSightings(width, height float64) {
	// Sharing draws no captions, and so sights nothing: a first sighting
	// nobody was shown is still owed (item 168).
	if g.hidden || g.share != nil || g.explainHeld() {
		return
	}
	g.captions = g.explainCaptions(false, true, width, height)
	g.sightingsQueued = true
	for _, c := range g.captions {
		text, plate := c.Text, c.Plate
		g.queueLabel(pendingLabel{
			mapLabel: mapLabel{Kind: labelCaption, Rect: plate.Rect, Anchor: plate.Rect.Center()},
			draw: func(screen *ebiten.Image, at city.Rect) {
				g.drawCaption(screen, text, plate.Moved(at.Min))
			},
			ink:   g.theme.Palette.Panel,
			faded: g.lit.RecedesGround() && c.Kind != city.KindCar,
		})
	}
}

// drawExplain draws explain mode's captions over everything, and the
// first sightings when the map did not lay them out with its labels.
func (g *Game) drawExplain(screen *ebiten.Image, width, height float64) {
	held := g.explainHeld()
	if !held && g.sightingsQueued {
		g.sightingsQueued = false
		return
	}
	g.captions = g.explainCaptions(held, false, width, height)
	for _, c := range g.captions {
		g.drawCaption(screen, c.Text, c.Plate)
	}
}

func (g *Game) drawCaption(screen *ebiten.Image, text string, p ui.Plate) {
	g.roundRect(screen, p.Rect, g.theme.Radius()/2, g.theme.Palette.Panel)
	g.text(screen, p.TextAt, text, p.Size, g.theme.Palette.Text)
}
