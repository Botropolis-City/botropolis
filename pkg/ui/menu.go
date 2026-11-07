package ui

import (
	"image/color"
	"math"

	"botropolis.city/botropolis/pkg/city"
)

// The action menu (item 350): three columns of verbs, each row with the
// key that runs it, and under them one line saying what the hovered or
// focused row does. It is placed at a point, the pointer or a button,
// and kept inside the window.

// MenuRow is one verb: its label, the key that runs it, and what it
// does. Path is the presses that run it, which end its description
// (item 351).
type MenuRow struct {
	Label, Key, Describe string
	Path                 []string
}

// MenuColumn is one column of the menu with the key that chooses it.
type MenuColumn struct {
	Title, Key string
	Rows       []MenuRow
}

// MenuFocus is the row the keyboard is on: a column and a row in it, or
// -1 for neither.
type MenuFocus struct{ Column, Row int }

// NoMenuFocus is a menu with nothing focused.
var NoMenuFocus = MenuFocus{Column: -1, Row: -1}

// MenuItem is one row as laid out: its plate, its label and its key.
type MenuItem struct {
	Rect  city.Rect
	Label Text
	Cap   KeyCap
}

// MenuHeading is a column's title and the key that chooses the column.
type MenuHeading struct {
	Title Text
	Cap   KeyCap
}

// Menu is the action menu as laid out.
type Menu struct {
	Rect        city.Rect
	Headings    []MenuHeading
	Rows        [][]MenuItem
	Description []Text
	// DescriptionDot and DescriptionCaps end the description with the
	// described row's path (item 351).
	DescriptionDot  Text
	DescriptionCaps []KeyCap
	Focus           MenuFocus
	// Fill is the plate's colour: opaque, since a menu opens over the
	// card, and the card's text showed through its rows.
	Fill color.NRGBA
}

// LayoutMenu lays the columns out with their top-left corner at a point,
// moved left or up as far as it takes to stay inside bounds. The
// description under the columns is the row under hover's, else the
// focused row's, and its room is reserved for the longest any row has,
// so the menu is one size whichever row is read.
func LayoutMenu(th Theme, columns []MenuColumn, focus MenuFocus, hover, at city.Point, bounds city.Rect, measure Measure) Menu {
	grid := th.Grid()
	pad, gap := 2*grid, 3*grid
	_, lineH := measure("", Body)
	_, smallH := measure("", Small)
	rowH := lineH + grid
	m := Menu{Focus: focus, Fill: th.Palette.Panel}
	m.Fill.A = 0xff
	x, top := pad, pad
	rowsTop := top + smallH + grid
	bottom := rowsTop
	for _, col := range columns {
		heading := MenuHeading{Title: Text{Text: col.Title, At: city.Point{X: x, Y: top}, Size: Small}}
		heading.Cap = LayoutKeyCap(th, col.Key, city.Point{}, measure)
		titleW, _ := measure(col.Title, Small)
		width := titleW + grid + heading.Cap.Rect.Width()
		var rows []MenuItem
		for i, row := range col.Rows {
			y := rowsTop + float64(i)*rowH
			item := MenuItem{Label: Text{Text: row.Label, At: city.Point{X: x + grid, Y: y + grid/2}, Size: Body}}
			item.Cap = LayoutKeyCap(th, row.Key, city.Point{}, measure)
			labelW, _ := measure(row.Label, Body)
			width = max(width, labelW+3*grid+item.Cap.Rect.Width())
			item.Rect = city.RectAt(x, y, 0, rowH)
			rows = append(rows, item)
		}
		for i := range rows {
			rows[i].Rect.Max.X = x + width
			rows[i].Cap = rows[i].Cap.Moved(city.Point{X: x + width - grid/2 - rows[i].Cap.Rect.Width(), Y: rows[i].Rect.Min.Y + (rowH-rows[i].Cap.Rect.Height())/2})
		}
		heading.Cap = heading.Cap.Moved(city.Point{X: x + width - heading.Cap.Rect.Width(), Y: top - grid/4})
		m.Headings = append(m.Headings, heading)
		m.Rows = append(m.Rows, rows)
		bottom = max(bottom, rowsTop+float64(len(rows))*rowH)
		x += width + gap
	}
	width := x - gap + pad
	if len(columns) == 0 {
		width = 2 * pad
	}
	textW := width - 2*pad
	reserved := 0
	for _, col := range columns {
		for _, row := range col.Rows {
			reserved = max(reserved, keyedRows(th, row.Describe, row.Path, textW, measure))
		}
	}
	descTop := bottom + grid
	height := descTop + float64(reserved)*smallH + pad
	if reserved == 0 {
		height = bottom + pad
	}
	m.Rect = city.RectAt(0, 0, width, height)

	// Place it, then read the hover against where it landed.
	to := at
	to.X = math.Min(to.X, bounds.Max.X-m.Rect.Width())
	to.Y = math.Min(to.Y, bounds.Max.Y-m.Rect.Height())
	to.X = math.Max(to.X, bounds.Min.X)
	to.Y = math.Max(to.Y, bounds.Min.Y)
	m = m.MoveTo(to)
	var described MenuRow
	if hit, ok := m.Hit(hover); ok {
		described = columns[hit.Column].Rows[hit.Row]
	} else if focus.Column >= 0 && focus.Column < len(columns) && focus.Row >= 0 && focus.Row < len(columns[focus.Column].Rows) {
		described = columns[focus.Column].Rows[focus.Row]
	}
	if described.Describe != "" || len(described.Path) > 0 {
		lines, keyRow, keyX := keyedLines(th, described.Describe, described.Path, textW, reserved, measure)
		row := func(i int) city.Point {
			return city.Point{X: m.Rect.Min.X + pad, Y: m.Rect.Min.Y + descTop + float64(i)*smallH}
		}
		for i, line := range lines {
			m.Description = append(m.Description, Text{Text: line, At: row(i), Size: Small})
		}
		if keyRow >= 0 && keyRow < reserved {
			m.DescriptionDot, m.DescriptionCaps = layKey(th, described.Path, row(keyRow), keyX, measure)
		}
	}
	return m
}

// Hit is the row under a point, if any.
func (m Menu) Hit(at city.Point) (MenuFocus, bool) {
	for c, rows := range m.Rows {
		for r, item := range rows {
			if item.Rect.Contains(at) {
				return MenuFocus{Column: c, Row: r}, true
			}
		}
	}
	return NoMenuFocus, false
}

// MoveTo puts the menu's top-left corner at a point, carrying everything
// on it.
func (m Menu) MoveTo(at city.Point) Menu {
	by := at.Sub(m.Rect.Min)
	moved := Menu{Rect: city.Rect{Min: at, Max: m.Rect.Max.Add(by)}, Focus: m.Focus, Fill: m.Fill}
	for _, h := range m.Headings {
		moved.Headings = append(moved.Headings, MenuHeading{Title: h.Title.moved(by), Cap: h.Cap.Moved(h.Cap.Rect.Min.Add(by))})
	}
	for _, rows := range m.Rows {
		var col []MenuItem
		for _, item := range rows {
			col = append(col, MenuItem{Rect: shifted(item.Rect, by), Label: item.Label.moved(by), Cap: item.Cap.Moved(item.Cap.Rect.Min.Add(by))})
		}
		moved.Rows = append(moved.Rows, col)
	}
	for _, line := range m.Description {
		moved.Description = append(moved.Description, line.moved(by))
	}
	if m.DescriptionDot.Text != "" {
		moved.DescriptionDot = m.DescriptionDot.moved(by)
	}
	for _, c := range m.DescriptionCaps {
		moved.DescriptionCaps = append(moved.DescriptionCaps, c.Moved(c.Rect.Min.Add(by)))
	}
	return moved
}

// shifted is a rect moved by an offset.
func shifted(r city.Rect, by city.Point) city.Rect {
	return city.Rect{Min: r.Min.Add(by), Max: r.Max.Add(by)}
}
