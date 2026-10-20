package render

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// strip draws the resource strip along the top from its pkg/ui layout and
// returns how tall it was; it keeps the layout so a click can find a chip.
func (g *Game) strip(screen *ebiten.Image, width float64) float64 {
	th := g.theme
	chips := ui.StripChips(g.scene.City().Summary())
	if g.viewing() {
		chips = ui.Drained(chips)
	}
	strip := ui.LayoutStrip(th, chips, width, g.faces.Measure)
	g.bar(screen, city.RectAt(0, 0, width, strip.Height), strip.Height)
	for _, chip := range strip.Chips {
		if chip.Tone != ui.ToneNone {
			centre := chip.Dot.Center()
			vector.FillCircle(screen, float32(centre.X), float32(centre.Y), float32(chip.Dot.Width()/2), th.Color(chip.Tone), true)
		}
		g.text(screen, chip.TextAt, chip.Text, strip.Size, th.Palette.Text)
	}
	g.mu.Lock()
	g.stripLayout = strip
	g.mu.Unlock()
	return strip.Height
}

// stripHover is the state chip under the pointer, if any.
func (g *Game) stripHover(at city.Point) (state.State, bool) {
	g.mu.Lock()
	strip := g.stripLayout
	g.mu.Unlock()
	chip, ok := strip.Hit(at)
	return chip.State, ok
}

// stripClick jumps to the next building of the state whose chip was
// clicked, and says so.
func (g *Game) stripClick(at city.Point) bool {
	g.mu.Lock()
	strip := g.stripLayout
	g.mu.Unlock()
	if chip, ok := strip.Hit(at); ok {
		g.jump(chip.State)
		return true
	}
	return at.Y < strip.Height
}

func (g *Game) jump(st state.State) {
	if b := g.scene.JumpTo(st); b != nil {
		g.SetStatus(city.JumpedTo(st, b.Card(g.scene.City().Time).Title))
	} else {
		g.SetStatus(city.NothingToJumpTo(st))
	}
}
