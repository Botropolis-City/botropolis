package ui

import (
	"fmt"
	"path/filepath"

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
	Rect city.Rect
	Rows []SidebarRow
}

const sidebarGrids = 36

// SidebarWidth is the panel's width at a scale.
func SidebarWidth(th Theme) float64 {
	return sidebarGrids * th.Grid()
}

// LayoutSidebar lists projects with their sessions, then the hidden
// projects, down a panel between the strip and the footer; rows that
// would run past the bottom are dropped.
func LayoutSidebar(th Theme, top, bottom float64, projects []city.ProjectRow, hidden []string, selectedID string, measure Measure) Sidebar {
	grid := th.Grid()
	sb := Sidebar{Rect: city.RectAt(0, top, SidebarWidth(th), bottom-top)}
	_, bodyH := measure("", Body)
	_, smallH := measure("", Small)
	rowH := bodyH + grid
	y := top + grid
	add := func(row SidebarRow, h float64) bool {
		if y+h > bottom-grid {
			return false
		}
		row.Rect = city.RectAt(0, y, sb.Rect.Width(), h)
		sb.Rows = append(sb.Rows, row)
		y += h
		return true
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
		row := SidebarRow{
			Kind:   RowProject,
			Root:   p.Root,
			Label:  Text{Text: label, At: city.Point{X: 2 * grid, Y: y + grid/2}, Size: Body},
			Detail: Text{Text: detail, At: city.Point{X: sb.Rect.Width() - 2*grid - dw, Y: y + grid/2 + (bodyH-smallH)/2}, Size: Small},
		}
		if !add(row, rowH) {
			return sb
		}
		for _, b := range p.Sessions {
			title := b.Session.Title
			if title == "" {
				title = state.ShortID(b.Session.ID)
			}
			tone := StateTone(b.Session.State)
			row := SidebarRow{
				Kind:     RowSession,
				Root:     p.Root,
				ID:       b.Session.ID,
				Tone:     tone,
				Dot:      city.RectAt(4*grid, y+(rowH-grid)/2, grid, grid),
				Label:    Text{Text: clipTo(title, sb.Rect.Width()-8*grid, Small, measure), At: city.Point{X: 6 * grid, Y: y + grid/2 + (bodyH-smallH)/2}, Size: Small},
				Selected: b.Session.ID == selectedID,
			}
			if !add(row, rowH) {
				return sb
			}
		}
	}
	if len(hidden) > 0 {
		if !add(SidebarRow{Kind: RowHeader, Label: Text{Text: "hidden", At: city.Point{X: 2 * grid, Y: y + grid}, Size: Small}}, rowH+grid/2) {
			return sb
		}
		for _, root := range hidden {
			row := SidebarRow{
				Kind:   RowHidden,
				Root:   root,
				Label:  Text{Text: filepath.Base(root), At: city.Point{X: 4 * grid, Y: y + grid/2}, Size: Body},
				Detail: Text{Text: "show", At: city.Point{X: sb.Rect.Width() - 2*grid - 4*7, Y: y + grid/2 + (bodyH-smallH)/2}, Size: Small},
			}
			if w, _ := measure("show", Small); w > 0 {
				row.Detail.At.X = sb.Rect.Width() - 2*grid - w
			}
			if !add(row, rowH) {
				return sb
			}
		}
	}
	return sb
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
