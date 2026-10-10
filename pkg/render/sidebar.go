package render

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// drawSidebar lists the projects down the left between the strip and
// the footer, with the selection's card docked at its foot, and keeps
// the layout for clicks.
func (g *Game) drawSidebar(screen *ebiten.Image, top, bottom float64) {
	th := g.theme
	c := g.scene.City()
	width := ui.SidebarWidth(th)
	g.bar(screen, city.RectAt(0, top, width, bottom-top), top)
	vector.FillRect(screen, float32(width-th.Hairline()), float32(top), float32(th.Hairline()), float32(bottom-top), th.Palette.Hairline, false)
	foot := bottom
	g.pinned = ui.Card{}
	g.pinnedKinds = nil
	if card, _, ok := g.scene.SelectedCard(); ok {
		laid := ui.LayoutCard(th, card, city.RectAt(0, top, width, bottom-top), g.faces.Measure)
		laid = laid.MoveTo(city.Point{X: th.Grid(), Y: bottom - th.Grid() - laid.Rect.Height()})
		foot = laid.Rect.Min.Y - th.Grid()
		g.card(screen, laid)
		g.pinned = laid
		g.pinnedKinds = g.scene.Actions()
	}
	selected := ""
	if b := g.scene.Selected(); b != nil {
		selected = b.Session.ID
	}
	sb := ui.LayoutSidebar(th, top, foot, c.Projects(g.scene.Layout()), g.scene.Layout().HiddenRoots(), selected, g.faces.Measure)
	for _, row := range sb.Rows {
		if row.Selected {
			g.roundRect(screen, row.Rect.Inset(th.Grid()/2), th.Radius()/2, th.Palette.Hairline)
		}
		if row.Kind == ui.RowSession {
			centre := row.Dot.Center()
			vector.FillCircle(screen, float32(centre.X), float32(centre.Y), float32(row.Dot.Width()/2), th.Color(row.Tone), true)
		}
		colour := th.Palette.Text
		if row.Kind == ui.RowSession || row.Kind == ui.RowHeader {
			colour = th.Palette.Dim
		}
		g.run(screen, row.Label, colour)
		if row.Detail.Text != "" {
			g.run(screen, row.Detail, th.Palette.Dim)
		}
	}
	g.mu.Lock()
	g.sidebarLayout = sb
	g.mu.Unlock()
}

// clickSidebar answers a click on the sidebar: a session row selects
// and centres it, a project row centres its district, a hidden row
// shows the project again; any click on the panel stops there.
func (g *Game) clickSidebar(at city.Point) bool {
	if !g.sidebar {
		return false
	}
	g.mu.Lock()
	sb := g.sidebarLayout
	g.mu.Unlock()
	if !sb.Rect.Contains(at) {
		return false
	}
	row, ok := sb.Hit(at)
	if !ok {
		return true
	}
	switch row.Kind {
	case ui.RowSession:
		g.scene.Select(row.ID)
	case ui.RowProject:
		g.scene.CenterOnProject(row.Root)
	case ui.RowHidden:
		g.scene.Unhide(row.Root)
		g.SetStatus("showing " + row.Root)
	}
	return true
}
