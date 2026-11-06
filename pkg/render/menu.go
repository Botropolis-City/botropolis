package render

import (
	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/ui"
)

// The action menu (item 350): the selection's verbs in three columns,
// Session, Project and Group, each row with the key that runs it. One
// panel serves the card's submenu buttons, the keyboard and a
// right-click, and the same table gives each row the key it shows and
// the key it answers, so the two cannot disagree.

// menuColumnKeys is the binding that chooses each of city.VerbColumns,
// in the same order.
var menuColumnKeys = []string{"menu_session", "menu_project", "menu_group"}

// menuKeys is the binding in config.Bindings each verb's row shows and
// answers, the one table (rules/two-things-that-must-agree.md): the menu
// reads a row's chord from it to draw it and to answer it, so a rebound
// letter moves both. Kinds that never show together share a binding:
// attach or resume, pack up or unpack, star or unstar. Demolish shows
// its key twice, since its second press confirms at the map. The
// columns themselves are city.VerbColumns, which the card's face reads.
var menuKeys = map[city.ActionKind]string{
	city.ActionAttach:    "menu_attach",
	city.ActionResume:    "menu_attach",
	city.ActionStop:      "menu_stop",
	city.ActionRead:      "menu_read",
	city.ActionGenerate:  "menu_generate",
	city.ActionFork:      "menu_fork",
	city.ActionPack:      "menu_pack",
	city.ActionUnpack:    "menu_pack",
	city.ActionBreakdown: "menu_breakdown",
	city.ActionOpenPR:    "menu_open_pr",
	city.ActionDemolish:  "menu_demolish",

	city.ActionNew:          "menu_new_here",
	city.ActionReveal:       "menu_reveal",
	city.ActionCopyPath:     "menu_copy_path",
	city.ActionStar:         "menu_star",
	city.ActionUnstar:       "menu_star",
	city.ActionHide:         "menu_hide",
	city.ActionDistrictType: "menu_district_type",
	city.ActionShowRoad:     "menu_show_road",

	city.ActionMoveToGroup:   "menu_move",
	city.ActionJoinSuggested: "menu_join",
	city.ActionTakeOut:       "menu_take_out",
	city.ActionRenameGroup:   "menu_rename_group",
	city.ActionCloseGroup:    "menu_close_group",
	city.ActionCopyStandup:   "menu_standup",
	city.ActionMuteWord:      "menu_mute",
	city.ActionFocusLine:     "menu_focus",
}

// menuColumns is city.VerbColumns cut down to the kinds offered, in
// their order: the columns for the layout, and beside them the kind
// each row runs and the chord that runs it, with each column's own.
func menuColumns(offered []city.ActionKind, describe func(city.ActionKind) string, chords config.Chords) ([]ui.MenuColumn, [][]city.ActionKind, [][]config.Chord, []config.Chord) {
	has := map[city.ActionKind]bool{}
	for _, k := range offered {
		has[k] = true
	}
	var columns []ui.MenuColumn
	var kinds [][]city.ActionKind
	var rowChords [][]config.Chord
	var colChords []config.Chord
	for i, col := range city.VerbColumns {
		column := ui.MenuColumn{Title: col.Title}
		var colChord config.Chord
		if i < len(menuColumnKeys) {
			colChord = chords.Bound[menuColumnKeys[i]]
			column.Key = colChord.Label()
		}
		var runs []city.ActionKind
		var keys []config.Chord
		for _, kind := range col.Kinds {
			if has[kind] {
				chord := chords.Bound[menuKeys[kind]]
				shown := chord.Label()
				if kind == city.ActionDemolish {
					shown += " " + shown
				}
				column.Rows = append(column.Rows, ui.MenuRow{Label: string(kind), Key: shown, Describe: describe(kind)})
				runs = append(runs, kind)
				keys = append(keys, chord)
			}
		}
		columns = append(columns, column)
		kinds = append(kinds, runs)
		rowChords = append(rowChords, keys)
		colChords = append(colChords, colChord)
	}
	return columns, kinds, rowChords, colChords
}

// pressedChord is a chord pressed this frame with exactly its modifiers.
func (g *Game) pressedChord(c config.Chord) bool {
	return c.Key != "" && g.chordPressed(c) && g.onlyHeld(c) && c.Shift == g.shifted()
}

// menuPanel is the menu as an overlay: it takes the keys and the pointer
// while it is up, and leaves on escape, a choice, or a click outside.
type menuPanel struct {
	columns []ui.MenuColumn
	kinds   [][]city.ActionKind
	// rowChords and colChords are the chords the rows and the columns
	// show, which are the chords they answer.
	rowChords [][]config.Chord
	colChords []config.Chord
	at        city.Point
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
	g.openMenuWith(offered, at, column, g.runVerb)
}

// menuFirstRows opens the menu armed on its first column that has rows,
// as a right-click does (item 350).
const menuFirstRows = -2

// openMenuWith opens the menu with what a chosen row does: the
// selection's verbs, or a district's, or a group's on the board.
func (g *Game) openMenuWith(offered []city.ActionKind, at city.Point, column int, choose func(city.ActionKind)) {
	if g.menu() != nil {
		return
	}
	columns, kinds, rowChords, colChords := menuColumns(offered, g.scene.DescribeVerb, g.findChords())
	if column == menuFirstRows {
		column = 0
		for i, k := range kinds {
			if len(k) > 0 {
				column = i
				break
			}
		}
	}
	p := &menuPanel{columns: columns, kinds: kinds, rowChords: rowChords, colChords: colChords, at: at, focus: ui.MenuFocus{Column: 0, Row: -1}, choose: choose}
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
	footer := ui.LayoutFooter(g.theme, width, height, footerKeys, measure)
	return ui.LayoutMenu(g.theme, p.columns, p.focus, g.pointer(), p.at, city.RectAt(0, 0, width, footer.Rect.Min.Y), measure)
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

// run chooses a row. The menu takes itself off the stack first, so a
// row that opens a panel of its own, such as move to group, leaves that
// panel on top; it reports true, since there is nothing left to pop.
func (p *menuPanel) run(g *Game, focus ui.MenuFocus) bool {
	if focus.Column < 0 || focus.Column >= len(p.kinds) || focus.Row < 0 || focus.Row >= len(p.kinds[focus.Column]) {
		return true
	}
	g.overlays.remove(p)
	p.choose(p.kinds[focus.Column][focus.Row])
	return true
}

func (p *menuPanel) keys(g *Game) bool {
	switch {
	case g.just(ebiten.KeyEscape):
		return false
	case g.just(ebiten.KeyEnter), g.just(ebiten.KeyKPEnter):
		return p.run(g, p.focus)
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
		for i, c := range p.colChords {
			if g.pressedChord(c) {
				p.focus, p.armed = ui.MenuFocus{Column: i, Row: 0}, true
				p.clampFocus()
				return true
			}
		}
		return true
	}
	if p.focus.Column >= 0 && p.focus.Column < len(p.rowChords) {
		for r, c := range p.rowChords[p.focus.Column] {
			if g.pressedChord(c) {
				return p.run(g, ui.MenuFocus{Column: p.focus.Column, Row: r})
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
		return p.run(g, hit)
	}
	return m.Rect.Contains(at)
}

func (p *menuPanel) draw(g *Game, screen *ebiten.Image, width, height float64) {
	th := g.theme
	m := p.layout(g, width, height)
	g.roundRect(screen, m.Rect, th.Radius(), m.Fill)
	g.roundStroke(screen, m.Rect, th.Radius(), th.Hairline(), th.Palette.Hairline)
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
