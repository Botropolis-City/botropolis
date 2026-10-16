package render

import (
	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// mutePicker lists the words that joined a session to its group, and
// mutes the one chosen, so it stops joining sessions (item 90).
type mutePicker struct {
	words    []string
	selected int
	rect     city.Rect
}

func (g *Game) mutePicker() *mutePicker {
	p, _ := g.overlays.top().(*mutePicker)
	return p
}

func (g *Game) openMutePicker(session string) {
	if g.mutePicker() != nil {
		return
	}
	for _, b := range g.scene.City().Buildings() {
		if b.Session.ID == session && len(b.Session.GroupWords) > 0 {
			g.overlays.push(&mutePicker{words: append([]string(nil), b.Session.GroupWords...)})
			return
		}
	}
	g.SetStatus("no word joined this session to its group")
}

func (p *mutePicker) keys(g *Game) bool {
	switch {
	case g.just(ebiten.KeyEscape):
		return false
	case g.just(ebiten.KeyEnter), g.just(ebiten.KeyKPEnter):
		word := p.words[p.selected]
		if g.actor != nil {
			if err := g.actor.Do(city.Action{Kind: city.ActionMute, Text: word}); err != nil {
				g.SetStatus(err.Error())
				return false
			}
		}
		g.SetStatus("muted “" + word + "”: groups again without it")
		return false
	case g.just(ebiten.KeyArrowDown):
		p.selected = min(p.selected+1, len(p.words)-1)
	case g.just(ebiten.KeyArrowUp):
		p.selected = max(0, p.selected-1)
	}
	return true
}

func (p *mutePicker) click(g *Game, at city.Point) bool {
	g.mu.Lock()
	panel := p.rect
	g.mu.Unlock()
	return panel.Contains(at)
}

func (p *mutePicker) draw(g *Game, screen *ebiten.Image, width, height float64) {
	th := g.theme
	grid := th.Grid()
	_, title := g.faces.Measure("", ui.Title)
	_, body := g.faces.Measure("", ui.Body)
	panelWidth := min(width-grid*8, grid*60)
	panel := city.RectAt((width-panelWidth)/2, grid*10, panelWidth, grid*4+title+grid+float64(len(p.words))*(body+grid/2))
	g.mu.Lock()
	p.rect = panel
	g.mu.Unlock()
	g.roundRect(screen, panel, th.Radius(), th.Palette.Panel)
	x, y := panel.Min.X+grid*2, panel.Min.Y+grid*2
	g.text(screen, city.Point{X: x, Y: y}, g.clip("mute a word   ↑ ↓ choose, enter mutes, esc cancels", panelWidth-grid*4, ui.Title), ui.Title, th.Palette.Text)
	y += title + grid
	for i, w := range p.words {
		if i == p.selected {
			g.roundRect(screen, city.RectAt(panel.Min.X+grid, y-grid/4, panelWidth-grid*2, body+grid/2), 0, th.Palette.Hairline)
		}
		g.text(screen, city.Point{X: x, Y: y}, g.clip(w, panelWidth-grid*4, ui.Body), ui.Body, th.Palette.Text)
		y += body + grid/2
	}
}
