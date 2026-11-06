package render

import (
	"image"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/exp/textinput"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Corrections from the map (item 87): move to group on a card opens a
// picker of the groups and no group, and a drag that starts on the
// selected building carries it to a line, whose group it joins.

const (
	noGroup  = "no group"
	newGroup = "new group…"
)

type picker struct {
	session string
	// ids and names are the rows, taken when the picker opens: new
	// group…, every group, then no group.
	ids      []string
	names    []string
	selected int
	rect     city.Rect

	// naming is set once new group… is chosen: the picker asks for the
	// new group's name in name, and refused says why enter did nothing
	// (item 288).
	naming bool
	// renaming is the group being renamed, when the panel was opened
	// to rename one rather than to move a session (item 333).
	renaming string
	// named, when set, takes the new name instead of the actor: the
	// walk keeps it until it is left (item 333).
	named   func(name string)
	name    field
	refused string
	ime     *textinput.Composer
	caret   city.Rect
}

func (g *Game) picker() *picker {
	p, _ := g.overlays.top().(*picker)
	return p
}

func (g *Game) openPicker(session string) {
	if g.picker() != nil {
		return
	}
	p := &picker{session: session, ids: []string{""}, names: []string{newGroup}}
	for _, gr := range g.scene.Groups() {
		p.ids, p.names = append(p.ids, gr.ID), append(p.names, gr.Name)
	}
	p.ids, p.names = append(p.ids, ""), append(p.names, noGroup)
	g.overlays.push(p)
}

// openRenamer opens the panel on a group's name, to rename it.
func (g *Game) openRenamer(id, name string) { g.openRenamerFor(id, name, nil) }

// openRenamerFor opens it handing the name to named, when given.
func (g *Game) openRenamerFor(id, name string, named func(string)) {
	if g.picker() != nil {
		return
	}
	p := &picker{renaming: id, named: named}
	g.startNaming(p)
	p.name.insert(name)
	g.overlays.push(p)
}

// renameRefusal is why a group cannot be renamed to the name typed: none,
// or one another group already has (item 288's rule, item 333).
func (g *Game) renameRefusal(id, name string) string {
	if name == "" {
		return "a group needs a name"
	}
	for _, gr := range g.scene.Groups() {
		if gr.ID != id && strings.EqualFold(strings.TrimSpace(gr.Name), name) {
			return gr.Name + " is already a group"
		}
	}
	return ""
}

func (p *picker) labels() []string {
	return p.names
}

func (p *picker) title() string {
	if p.renaming != "" {
		return "rename group   type a name, enter keeps it, esc cancels"
	}
	if p.naming {
		return "new group   type a name, enter makes it, esc goes back"
	}
	return "move to group   ↑ ↓ choose, enter moves, new group… makes one, esc cancels"
}

// commit and compose are where typing reaches the name, from the input
// method or, under test, the scripted rune.
func (p *picker) commit(s string) {
	p.name.insert(s)
	p.refused = ""
}

func (p *picker) compose(s string) {
	p.name.compose(s)
}

func (g *Game) startNaming(p *picker) {
	p.naming, p.name, p.refused = true, field{}, ""
	if !g.ime {
		return
	}
	p.ime = &textinput.Composer{
		OnNewSession: func() *textinput.SessionOptions {
			g.mu.Lock()
			caret := p.caret
			g.mu.Unlock()
			return &textinput.SessionOptions{
				CaretBounds:     image.Rect(int(caret.Min.X), int(caret.Min.Y), int(caret.Max.X)+1, int(caret.Max.Y)),
				TextBeforeCaret: string(p.name.text[:p.name.caret]),
				TextAfterCaret:  string(p.name.text[p.name.caret:]),
			}
		},
		OnComposition: func(c *textinput.Composition) { p.compose(c.Text()) },
		OnCommit:      func(c *textinput.Commit) { p.commit(c.Text()) },
	}
}

func (p *picker) stopNaming() {
	if p.ime != nil {
		p.ime.Cancel()
	}
	p.naming, p.ime, p.name, p.refused = false, nil, field{}, ""
}

// refusal is why a new group cannot have the name typed: none, or one
// a group already has, ignoring case and surrounding spaces.
func (g *Game) refusal(name string) string {
	if name == "" {
		return "a new group needs a name"
	}
	for _, gr := range g.scene.Groups() {
		if strings.EqualFold(strings.TrimSpace(gr.Name), name) {
			return gr.Name + " is already a group"
		}
	}
	return ""
}

func (p *picker) nameKeys(g *Game) bool {
	if p.ime != nil {
		if handled, err := p.ime.Update(); err != nil {
			g.failed(err)
		} else if handled {
			return true
		}
	}
	switch {
	case g.just(ebiten.KeyEscape):
		p.stopNaming()
		return p.renaming == ""
	case p.renaming != "" && (g.just(ebiten.KeyEnter) || g.just(ebiten.KeyKPEnter)):
		name := strings.TrimSpace(p.name.query())
		if p.refused = g.renameRefusal(p.renaming, name); p.refused != "" {
			return true
		}
		p.stopNaming()
		if p.named != nil {
			p.named(name)
			return false
		}
		g.act(city.Action{Kind: city.ActionRename, Group: p.renaming, GroupName: name})
		g.Notify("renamed the group "+name, ui.ToneNone, "")
		return false
	case g.just(ebiten.KeyEnter), g.just(ebiten.KeyKPEnter):
		name := strings.TrimSpace(p.name.query())
		if p.refused = g.refusal(name); p.refused != "" {
			return true
		}
		p.stopNaming()
		g.act(city.Action{Kind: city.ActionMove, SessionID: p.session, GroupName: name})
		g.Notify("moved to "+name+", a new group", ui.ToneNone, "")
		return false
	case g.just(ebiten.KeyBackspace):
		p.name.backspace()
		p.refused = ""
	}
	if g.scriptedRune != 0 {
		p.commit(string(g.scriptedRune))
	}
	if s, ok := g.pasted(p); ok {
		p.commit(s)
	}
	return true
}

func (p *picker) keys(g *Game) bool {
	if p.naming {
		return p.nameKeys(g)
	}
	switch g.listKeys(&p.selected, len(p.ids), listShape{}) {
	case listBack:
		return false
	case listChoose:
		if p.selected == 0 {
			g.startNaming(p)
			return true
		}
		g.move(p.session, p.ids[p.selected])
		return false
	}
	return true
}

func (p *picker) click(g *Game, at city.Point) bool {
	g.mu.Lock()
	panel := p.rect
	g.mu.Unlock()
	if !panel.Contains(at) {
		p.stopNaming()
		return false
	}
	return true
}

func (p *picker) draw(g *Game, screen *ebiten.Image, width, height float64) {
	th := g.theme
	labels := p.names
	grid := th.Grid()
	_, title := g.faces.Measure("", ui.Title)
	_, body := g.faces.Measure("", ui.Body)
	panelWidth := min(width-grid*8, grid*80)
	rows := len(labels)
	if p.naming {
		rows = 2
	}
	panel := city.RectAt((width-panelWidth)/2, grid*10, panelWidth, grid*4+title+grid+float64(rows)*(body+grid/2))
	g.mu.Lock()
	p.rect = panel
	g.mu.Unlock()
	g.roundRect(screen, panel, th.Radius(), th.Palette.Panel)
	x, y := panel.Min.X+grid*2, panel.Min.Y+grid*2
	g.text(screen, city.Point{X: x, Y: y}, g.clip(p.title(), panelWidth-grid*4, ui.Title), ui.Title, th.Palette.Text)
	y += title + grid
	if p.naming {
		g.roundRect(screen, city.RectAt(panel.Min.X+grid, y-grid/4, panelWidth-grid*2, body+grid/2), 0, th.Palette.Hairline)
		shown, at := p.name.shown()
		before, _ := g.faces.Measure(string([]rune(shown)[:at]), ui.Body)
		caret := city.RectAt(x+before, y, 2, body)
		g.mu.Lock()
		p.caret = caret
		g.mu.Unlock()
		g.text(screen, city.Point{X: x, Y: y}, shown, ui.Body, th.Palette.Text)
		g.roundRect(screen, caret, 0, th.Palette.Accent)
		y += body + grid/2
		if p.refused != "" {
			g.text(screen, city.Point{X: x, Y: y}, p.refused, ui.Body, th.Palette.Error)
		}
		return
	}
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
	g.Notify("moved to "+where, ui.ToneNone, "")
}

// carries reports whether a drag that starts on hit carries the selected
// building rather than panning: only when it starts on the selection.
func carries(hit city.Hit, selected *city.Building) bool {
	return selected != nil && hit.Building == selected
}

// dropAction is what dropping a carried building on hit does: on a
// line's link, or on another building in a group, it joins that group;
// anywhere else nothing (item 348).
func dropAction(b *city.Building, hit city.Hit) city.Action {
	switch {
	case hit.Station != nil && hit.Station.Line != nil && hit.Station.Line.Group != nil:
		return city.Action{Kind: city.ActionMove, SessionID: b.Session.ID, Group: hit.Station.Line.Group.ID}
	case hit.Building != nil && hit.Building != b && hit.Building.Group != nil:
		return city.Action{Kind: city.ActionMove, SessionID: b.Session.ID, Group: hit.Building.Group.ID}
	}
	return city.Action{}
}

// drawCarry is the tie from a carried building to the pointer.
func (g *Game) drawCarry(screen *ebiten.Image, cam *city.Camera, cursor city.Point) {
	if g.carrying == nil {
		return
	}
	from := cam.WorldToScreen(g.carrying.Rect.Center())
	g.dashed(screen, from, cursor, 2, 6, colorLineInk)
	g.fillCircle(screen, float32(cursor.X), float32(cursor.Y), 4, colorLineInk)
}
