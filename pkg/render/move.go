package render

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Corrections from the map (item 87): move to group on a card opens a
// picker of the groups and no group, and a drag that starts on the
// selected building carries it to a line, whose group it joins.

const noGroup = "no group"

type picker struct {
	session string
	// ids and names are the rows, taken when the picker opens: every
	// group, then no group.
	ids      []string
	names    []string
	selected int
	rect     city.Rect
}

func (g *Game) picker() *picker {
	p, _ := g.overlays.top().(*picker)
	return p
}

func (g *Game) openPicker(session string) {
	if g.picker() != nil {
		return
	}
	p := &picker{session: session}
	for _, gr := range g.scene.Groups() {
		p.ids, p.names = append(p.ids, gr.ID), append(p.names, gr.Name)
	}
	p.ids, p.names = append(p.ids, ""), append(p.names, noGroup)
	g.overlays.push(p)
}

func (p *picker) labels() []string {
	return p.names
}

func (p *picker) keys(g *Game) bool {
	switch {
	case g.just(ebiten.KeyEscape):
		return false
	case g.just(ebiten.KeyEnter), g.just(ebiten.KeyKPEnter):
		g.move(p.session, p.ids[p.selected])
		return false
	case g.just(ebiten.KeyArrowDown):
		p.selected = min(p.selected+1, len(p.ids)-1)
	case g.just(ebiten.KeyArrowUp):
		p.selected = max(0, p.selected-1)
	}
	return true
}

func (p *picker) click(g *Game, at city.Point) bool {
	g.mu.Lock()
	panel := p.rect
	g.mu.Unlock()
	return panel.Contains(at)
}

func (p *picker) draw(g *Game, screen *ebiten.Image, width, height float64) {
	th := g.theme
	labels := p.names
	grid := th.Grid()
	_, title := g.faces.Measure("", ui.Title)
	_, body := g.faces.Measure("", ui.Body)
	panelWidth := min(width-grid*8, grid*60)
	panel := city.RectAt((width-panelWidth)/2, grid*10, panelWidth, grid*4+title+grid+float64(len(labels))*(body+grid/2))
	g.mu.Lock()
	p.rect = panel
	g.mu.Unlock()
	g.roundRect(screen, panel, th.Radius(), th.Palette.Panel)
	x, y := panel.Min.X+grid*2, panel.Min.Y+grid*2
	g.text(screen, city.Point{X: x, Y: y}, "move to group   ↑ ↓ choose, enter moves, esc cancels", ui.Title, th.Palette.Text)
	y += title + grid
	for i, label := range labels {
		if i == p.selected {
			g.roundRect(screen, city.RectAt(panel.Min.X+grid, y-grid/4, panelWidth-grid*2, body+grid/2), 0, th.Palette.Hairline)
		}
		g.text(screen, city.Point{X: x, Y: y}, g.clip(label, panelWidth-grid*4, ui.Body), ui.Body, th.Palette.Text)
		y += body + grid/2
	}
}

// move hands a session's move to the actor and says where it went.
func (g *Game) move(session, to string) {
	g.act(city.Action{Kind: city.ActionMove, SessionID: session, Group: to})
	where := noGroup
	for _, gr := range g.scene.Groups() {
		if gr.ID == to {
			where = gr.Name
		}
	}
	g.SetStatus("moved to " + where)
}

// carries reports whether a drag that starts on hit carries the selected
// building rather than panning: only when it starts on the selection.
func carries(hit city.Hit, selected *city.Building) bool {
	return selected != nil && hit.Building == selected
}

// dropAction is what dropping a carried building on hit does: on a
// line's station it joins that line's group, anywhere else nothing.
func dropAction(b *city.Building, hit city.Hit) city.Action {
	if hit.Station == nil || hit.Station.Line == nil || hit.Station.Line.Group == nil {
		return city.Action{}
	}
	return city.Action{Kind: city.ActionMove, SessionID: b.Session.ID, Group: hit.Station.Line.Group.ID}
}

// drawCarry is the tie from a carried building to the pointer.
func (g *Game) drawCarry(screen *ebiten.Image, cam *city.Camera, cursor city.Point) {
	if g.carrying == nil {
		return
	}
	from := cam.WorldToScreen(g.carrying.Rect.Center())
	dashed(screen, from, cursor, 2, 6, colorLineInk)
	vector.FillCircle(screen, float32(cursor.X), float32(cursor.Y), 4, colorLineInk, true)
}
