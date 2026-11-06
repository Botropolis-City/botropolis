package render

import (
	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// strip draws the resource strip along the top from its pkg/ui layout and
// returns how tall it was; it keeps the layout so a click can find a chip.
func (g *Game) strip(screen *ebiten.Image, width float64) float64 {
	th := g.theme
	chips := ui.StripChips(g.scene.Summary())
	if g.viewing() {
		chips = ui.Drained(chips)
	}
	strip := ui.LayoutStrip(th, chips, width, g.faces.Measure)
	g.bar(screen, city.RectAt(0, 0, width, strip.Height), strip.Height)
	// Every dot, then every label: they never overlap, and drawn apart
	// the dots go down together and the labels together, where taking
	// them chip by chip made a draw of each (item 193).
	pointer := g.pointer()
	waitingHovered := false
	for _, chip := range strip.Chips {
		if chip.Button {
			// The needs-you count is a button: a plate under it, brighter
			// under the pointer, as the footer's needs-you line was. The
			// plate goes down first, so its dot keeps its state's tone.
			waitingHovered = chip.Rect.Contains(pointer)
			fill := th.Palette.Hairline
			if waitingHovered {
				fill = buttonHover(fill)
			}
			plate := city.RectAt(chip.Rect.Min.X, chip.Rect.Min.Y+th.Grid()/2, chip.Rect.Width(), chip.Rect.Height()-th.Grid())
			g.roundRect(screen, plate, th.Radius()/2, fill)
		}
	}
	for _, chip := range strip.Chips {
		if chip.Tone != ui.ToneNone {
			centre := chip.Dot.Center()
			g.fillCircle(screen, float32(centre.X), float32(centre.Y), float32(chip.Dot.Width()/2), th.Color(chip.Tone))
		}
	}
	for _, chip := range strip.Chips {
		tone := th.Palette.Text
		if chip.Button {
			tone = th.Palette.NeedsYou
		}
		g.text(screen, chip.TextAt, chip.Text, strip.Size, tone)
	}
	g.mu.Lock()
	g.stripLayout, g.waitingHovered = strip, waitingHovered
	g.mu.Unlock()
	return strip.Height
}

// waitingChip is the needs-you count as last drawn, and the session a
// click on it attaches: who has waited longest, in triage's order.
func (g *Game) waitingChip() (city.Rect, *city.Building) {
	g.mu.Lock()
	strip := g.stripLayout
	g.mu.Unlock()
	walk := g.scene.TriageWalk()
	for _, c := range strip.Chips {
		if c.Button && len(walk) > 0 {
			return c.Rect, walk[0]
		}
	}
	return city.Rect{}, nil
}

// stripHover is the state chip under the pointer, if any.
func (g *Game) stripHover(at city.Point) (state.State, bool) {
	g.mu.Lock()
	strip := g.stripLayout
	g.mu.Unlock()
	chip, ok := strip.Hit(at)
	return chip.State, ok
}

// stripClick takes a click on the strip: the needs-you count attaches the
// session that has waited longest, in one click (items 91, 352), and any
// other count jumps to the next building in its state, and says so.
func (g *Game) stripClick(at city.Point) bool {
	g.mu.Lock()
	strip := g.stripLayout
	g.mu.Unlock()
	if chip, ok := strip.Hit(at); ok {
		if chip.Button {
			if _, b := g.waitingChip(); b != nil {
				g.act(city.Action{Kind: city.ActionAttach, SessionID: b.Session.ID})
			}
			return true
		}
		g.jump(chip.State)
		return true
	}
	return at.Y < strip.Height
}

func (g *Game) jump(st state.State) {
	if b := g.scene.JumpTo(st); b != nil {
		g.notify(ui.Message{Text: city.JumpedTo(st, b.Card(g.scene.City().Time).Title), Tone: ui.StateTone(st), SessionID: b.Session.ID})
	} else {
		g.Notify(city.NothingToJumpTo(st), ui.ToneNone, "")
	}
}
