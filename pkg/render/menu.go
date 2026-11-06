package render

import (
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// The action menu (item 350): the selection's verbs in three columns,
// Session, Project and Group, each row with the key that runs it. One
// panel serves the card's submenu buttons, the keyboard and a
// right-click, and the same table gives each row the key it shows and
// the key it answers, so the two cannot disagree.

// menuColumnKeys is the key that chooses each of city.VerbColumns, in
// the same order.
var menuColumnKeys = []string{"s", "p", "g"}

// menuKeys is the key each verb's row shows and answers, the one table
// (rules/two-things-that-must-agree.md): the menu reads a row's key from
// here to draw it and from here to answer it. Letters are unique within
// a column among the kinds that can show together, and mnemonic where
// they can be; "d d" is demolish's, whose second press confirms at the
// map. The columns themselves are city.VerbColumns, which the card's
// face reads too.
var menuKeys = map[city.ActionKind]string{
	city.ActionAttach:    "a",
	city.ActionResume:    "a",
	city.ActionStop:      "s",
	city.ActionRead:      "r",
	city.ActionGenerate:  "g",
	city.ActionFork:      "f",
	city.ActionPack:      "p",
	city.ActionUnpack:    "p",
	city.ActionBreakdown: "b",
	city.ActionOpenPR:    "o",
	city.ActionDemolish:  "d d",

	city.ActionNew:          "n",
	city.ActionReveal:       "f",
	city.ActionCopyPath:     "c",
	city.ActionStar:         "s",
	city.ActionUnstar:       "s",
	city.ActionHide:         "h",
	city.ActionDistrictType: "d",
	city.ActionShowRoad:     "r",

	city.ActionMoveToGroup:   "m",
	city.ActionJoinSuggested: "a",
	city.ActionTakeOut:       "t",
	city.ActionRenameGroup:   "n",
	city.ActionCloseGroup:    "c",
	city.ActionCopyStandup:   "s",
	city.ActionMuteWord:      "w",
	city.ActionFocusLine:     "f",
}

// menuColumns is city.VerbColumns cut down to the kinds offered, in
// their order: the columns for the layout, and beside them the kind
// each row runs.
func menuColumns(offered []city.ActionKind, describe func(city.ActionKind) string) ([]ui.MenuColumn, [][]city.ActionKind) {
	has := map[city.ActionKind]bool{}
	for _, k := range offered {
		has[k] = true
	}
	var columns []ui.MenuColumn
	var kinds [][]city.ActionKind
	for i, col := range city.VerbColumns {
		column := ui.MenuColumn{Title: col.Title}
		if i < len(menuColumnKeys) {
			column.Key = menuColumnKeys[i]
		}
		var runs []city.ActionKind
		for _, kind := range col.Kinds {
			if has[kind] {
				column.Rows = append(column.Rows, ui.MenuRow{Label: string(kind), Key: menuKeys[kind], Describe: describe(kind)})
				runs = append(runs, kind)
			}
		}
		columns = append(columns, column)
		kinds = append(kinds, runs)
	}
	return columns, kinds
}

// menuKey is the key a row or a column shows, as the keyboard presses
// it: the first key of what is shown, so "d d" answers to d.
func menuKey(shown string) (ebiten.Key, bool) {
	fields := strings.Fields(shown)
	if len(fields) == 0 {
		return -1, false
	}
	return keyByName(fields[0])
}

// menuPanel is the menu as an overlay: it takes the keys and the pointer
// while it is up, and leaves on escape, a choice, or a click outside.
type menuPanel struct {
	columns []ui.MenuColumn
	kinds   [][]city.ActionKind
	at      city.Point
	// focus is the column the keyboard is in and the row in it, -1 for
	// none; armed is whether a row's letter alone answers, which it does
	// once a column was chosen, by its letter, a button or the arrows.
	focus ui.MenuFocus
	armed bool
	// choose runs the chosen verb; it is the verb's path, swapped out
	// under test.
	choose func(kind city.ActionKind)
}

// openMenu opens the menu for the kinds offered at a point of the
// window: on a column, focused on its first row, or with column -1 on
// the first column with nothing chosen, as the keyboard opens it.
func (g *Game) openMenu(offered []city.ActionKind, at city.Point, column int) {
	if g.menu() != nil {
		return
	}
	columns, kinds := menuColumns(offered, g.scene.DescribeVerb)
	p := &menuPanel{columns: columns, kinds: kinds, at: at, focus: ui.MenuFocus{Column: 0, Row: -1}, choose: g.runVerb}
	if column >= 0 && column < len(columns) {
		p.focus, p.armed = ui.MenuFocus{Column: column, Row: 0}, true
	}
	p.clampFocus()
	g.overlays.push(p)
}

func (g *Game) menu() *menuPanel {
	p, _ := g.overlays.top().(*menuPanel)
	return p
}

// runVerb runs one of the selection's verbs the way the card's buttons
// do: the scene acts, the note is shown, and the action goes to the
// actor. Demolish comes back as a question, which act asks through the
// one dialog for destroying things, as d does (item 352).
func (g *Game) runVerb(kind city.ActionKind) {
	action, note := g.scene.Act(kind)
	// What runs beside the loop says so on its button (item 352); every
	// other note is a notice.
	if note != "" && !runsClaude[action.Kind] {
		g.Notify(note, ui.ToneNone, "")
	}
	g.act(action)
}

func (p *menuPanel) layout(g *Game, width, height float64) ui.Menu {
	_, _, measure := g.metrics()
	return ui.LayoutMenu(g.theme, p.columns, p.focus, g.pointer(), p.at, city.RectAt(0, 0, width, height), measure)
}

// rows is how many rows the focused column has.
func (p *menuPanel) rows() int {
	if p.focus.Column < 0 || p.focus.Column >= len(p.columns) {
		return 0
	}
	return len(p.columns[p.focus.Column].Rows)
}

// clampFocus keeps the focused row inside its column, and on none when
// the column has none.
func (p *menuPanel) clampFocus() {
	if n := p.rows(); n == 0 {
		p.focus.Row = -1
	} else if p.focus.Row >= n {
		p.focus.Row = n - 1
	}
}

// step moves the focus down or up a row, choosing the column on the
// way in.
func (p *menuPanel) step(by int) {
	p.armed = true
	if p.focus.Row < 0 {
		p.focus.Row = 0
	} else {
		p.focus.Row = max(0, p.focus.Row+by)
	}
	p.clampFocus()
}

// turn moves the focus to the next or previous column, wrapping.
func (p *menuPanel) turn(by int) {
	if len(p.columns) == 0 {
		return
	}
	p.focus.Column = (p.focus.Column + by + len(p.columns)) % len(p.columns)
	p.focus.Row = 0
	p.armed = true
	p.clampFocus()
}

// run chooses a row and reports that the menu closes.
func (p *menuPanel) run(focus ui.MenuFocus) bool {
	if focus.Column < 0 || focus.Column >= len(p.kinds) || focus.Row < 0 || focus.Row >= len(p.kinds[focus.Column]) {
		return true
	}
	p.choose(p.kinds[focus.Column][focus.Row])
	return false
}

func (p *menuPanel) keys(g *Game) bool {
	switch {
	case g.just(ebiten.KeyEscape):
		return false
	case g.just(ebiten.KeyEnter), g.just(ebiten.KeyKPEnter):
		return p.run(p.focus)
	case g.just(ebiten.KeyArrowDown), g.just(ebiten.KeyJ):
		p.step(1)
		return true
	case g.just(ebiten.KeyArrowUp), g.just(ebiten.KeyK):
		p.step(-1)
		return true
	case g.just(ebiten.KeyArrowRight), g.just(ebiten.KeyTab):
		p.turn(1)
		return true
	case g.just(ebiten.KeyArrowLeft):
		p.turn(-1)
		return true
	}
	if !p.armed {
		for i, col := range p.columns {
			if key, ok := menuKey(col.Key); ok && g.just(key) {
				p.focus, p.armed = ui.MenuFocus{Column: i, Row: 0}, true
				p.clampFocus()
				return true
			}
		}
		return true
	}
	if p.focus.Column >= 0 && p.focus.Column < len(p.columns) {
		for r, row := range p.columns[p.focus.Column].Rows {
			if key, ok := menuKey(row.Key); ok && g.just(key) {
				return p.run(ui.MenuFocus{Column: p.focus.Column, Row: r})
			}
		}
	}
	return true
}

// click runs the row under the pointer, stays on a press anywhere else
// on the panel, and closes on one outside it, which goes no further.
func (p *menuPanel) click(g *Game, at city.Point) bool {
	size := g.scene.Size()
	m := p.layout(g, size.X, size.Y)
	if hit, ok := m.Hit(at); ok {
		return p.run(hit)
	}
	return m.Rect.Contains(at)
}

func (p *menuPanel) draw(g *Game, screen *ebiten.Image, width, height float64) {
	th := g.theme
	m := p.layout(g, width, height)
	g.roundPanel(screen, m.Rect)
	pointer := g.pointer()
	for c, heading := range m.Headings {
		tone := th.Palette.Dim
		if c == p.focus.Column {
			tone = th.Palette.Text
		}
		g.run(screen, heading.Title, tone)
		g.keyCap(screen, heading.Cap)
	}
	for c, rows := range m.Rows {
		for r, item := range rows {
			focused := p.armed && p.focus == (ui.MenuFocus{Column: c, Row: r})
			if focused || item.Rect.Contains(pointer) {
				fill := th.Palette.Hairline
				if item.Rect.Contains(pointer) {
					fill = buttonHover(fill)
				}
				g.roundRect(screen, item.Rect, th.Radius()/2, fill)
			}
			g.run(screen, item.Label, th.Palette.Text)
			g.keyCap(screen, item.Cap)
		}
	}
	for _, line := range m.Description {
		g.run(screen, line, th.Palette.Dim)
	}
}
