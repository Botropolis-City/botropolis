package ui

import (
	"fmt"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// RowKind says what a sidebar row stands for.
type RowKind int

const (
	RowProject RowKind = iota
	RowSession
	RowHeader
	RowHidden
)

// SidebarRow is one line of the sidebar and what clicking it means.
type SidebarRow struct {
	Kind     RowKind
	Rect     city.Rect
	Label    Text
	Detail   Text
	Tone     Tone
	Dot      city.Rect
	Root     string
	ID       string
	Selected bool
}

// Sidebar is the project list down the left edge.
type Sidebar struct {
	Rect         city.Rect
	Rows         []SidebarRow
	Skip, Max    int
	Page         int
	Above, Below Text
}

const sidebarGrids = 36

// SidebarWidth is the panel's width at a scale.
func SidebarWidth(th Theme) float64 {
	return sidebarGrids * th.Grid()
}

// LayoutSidebar lists projects with their sessions, then the hidden
// projects, down a panel between the strip and the footer. Rows that do
// not fit scroll by whole rows: skip is how many are scrolled past, and
// the layout keeps it in range and marks how many are above and below.
// With reveal, it scrolls just far enough to show the selected session.
func LayoutSidebar(th Theme, top, bottom float64, projects []city.ProjectRow, hidden []city.ProjectRow, selectedID string, skip int, reveal bool, measure Measure) Sidebar {
	grid := th.Grid()
	sb := Sidebar{Rect: city.RectAt(0, top, SidebarWidth(th), bottom-top)}
	_, bodyH := measure("", Body)
	_, smallH := measure("", Small)
	rowH := bodyH + grid
	rows := sidebarRows(th, sb.Rect.Width(), projects, hidden, selectedID, measure)

	room := bottom - top - 2*grid
	markH := rowH
	fits := func(first int, h float64) int {
		used, n := 0.0, 0
		for _, r := range rows[first:] {
			if used+r.Rect.Height() > h {
				break
			}
			used += r.Rect.Height()
			n++
		}
		return n
	}
	shown := func(skip int) int {
		h := room
		if skip > 0 {
			h -= markH
		}
		n := fits(skip, h)
		if skip+n < len(rows) {
			n = fits(skip, h-markH)
		}
		return n
	}
	sb.Max = 0
	for s := range rows {
		h := room
		if s > 0 {
			h -= markH
		}
		if fits(s, h) == len(rows)-s {
			sb.Max = s
			break
		}
	}
	skip = min(max(skip, 0), sb.Max)
	if reveal {
		for i, r := range rows {
			if !r.Selected {
				continue
			}
			if i < skip {
				skip = i
			}
			for skip < sb.Max && i >= skip+shown(skip) {
				skip++
			}
		}
	}
	sb.Skip = skip
	sb.Page = max(fits(skip, room-2*markH), 1)

	y := top + grid
	markAt := func(y float64) city.Point {
		return city.Point{X: 2 * grid, Y: y + grid/2 + (bodyH-smallH)/2}
	}
	if skip > 0 {
		sb.Above = Text{Text: fmt.Sprintf("%d more above", skip), At: markAt(y), Size: Small}
		y += markH
	}
	n := shown(skip)
	for _, r := range rows[skip : skip+n] {
		sb.Rows = append(sb.Rows, r.moved(y))
		y += r.Rect.Height()
	}
	if rest := len(rows) - skip - n; rest > 0 {
		sb.Below = Text{Text: fmt.Sprintf("%d more below", rest), At: markAt(bottom - grid - markH), Size: Small}
	}
	return sb
}

// sidebarRows lays every row out from y zero, to be moved into place.
func sidebarRows(th Theme, width float64, projects []city.ProjectRow, hidden []city.ProjectRow, selectedID string, measure Measure) []SidebarRow {
	grid := th.Grid()
	_, bodyH := measure("", Body)
	_, smallH := measure("", Small)
	rowH := bodyH + grid
	var rows []SidebarRow
	add := func(row SidebarRow, h float64) {
		row.Rect = city.RectAt(0, 0, width, h)
		rows = append(rows, row)
	}
	for _, p := range projects {
		label := p.Name
		if p.Starred {
			label = "★ " + p.Name
		}
		detail := fmt.Sprintf("%d live", p.Live)
		if p.NeedsYou > 0 {
			detail = fmt.Sprintf("%d need you · %d live", p.NeedsYou, p.Live)
		}
		dw, _ := measure(detail, Small)
		add(SidebarRow{
			Kind:   RowProject,
			Root:   p.Root,
			Label:  Text{Text: label, At: city.Point{X: 2 * grid, Y: grid / 2}, Size: Body},
			Detail: Text{Text: detail, At: city.Point{X: width - 2*grid - dw, Y: grid/2 + (bodyH-smallH)/2}, Size: Small},
		}, rowH)
		for _, b := range p.Sessions {
			title := b.Session.Title
			if title == "" {
				title = state.ShortID(b.Session.ID)
			}
			add(SidebarRow{
				Kind:     RowSession,
				Root:     p.Root,
				ID:       b.Session.ID,
				Tone:     StateTone(b.Session.State),
				Dot:      city.RectAt(4*grid, (rowH-grid)/2, grid, grid),
				Label:    Text{Text: clipTo(title, width-8*grid, Small, measure), At: city.Point{X: 6 * grid, Y: grid/2 + (bodyH-smallH)/2}, Size: Small},
				Selected: b.Session.ID == selectedID,
			}, rowH)
		}
	}
	if len(hidden) > 0 {
		add(SidebarRow{Kind: RowHeader, Label: Text{Text: "hidden", At: city.Point{X: 2 * grid, Y: grid}, Size: Small}}, rowH+grid/2)
		for _, h := range hidden {
			row := SidebarRow{
				Kind:   RowHidden,
				Root:   h.Root,
				Label:  Text{Text: h.Name, At: city.Point{X: 4 * grid, Y: grid / 2}, Size: Body},
				Detail: Text{Text: "show", At: city.Point{X: width - 2*grid - 4*7, Y: grid/2 + (bodyH-smallH)/2}, Size: Small},
			}
			if w, _ := measure("show", Small); w > 0 {
				row.Detail.At.X = width - 2*grid - w
			}
			add(row, rowH)
		}
	}
	return rows
}

// moved is the row with its top at y.
func (r SidebarRow) moved(y float64) SidebarRow {
	dy := y - r.Rect.Min.Y
	r.Rect = city.RectAt(r.Rect.Min.X, y, r.Rect.Width(), r.Rect.Height())
	r.Label.At.Y += dy
	r.Detail.At.Y += dy
	if r.Dot.Area() > 0 {
		r.Dot = city.RectAt(r.Dot.Min.X, r.Dot.Min.Y+dy, r.Dot.Width(), r.Dot.Height())
	}
	return r
}

// Hit is the row under a point, if any.
func (s Sidebar) Hit(at city.Point) (SidebarRow, bool) {
	for _, r := range s.Rows {
		if r.Rect.Contains(at) {
			return r, true
		}
	}
	return SidebarRow{}, false
}
