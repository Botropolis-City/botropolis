package render

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"

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
	g.paint.Box(screen, float32(width-th.Hairline()), float32(top), float32(th.Hairline()), float32(bottom-top), th.Palette.Hairline)
	foot := bottom
	g.pinned = ui.Card{}
	g.pinnedKinds = nil
	g.cardOver = ui.Card{}
	if card, _, ok := g.scene.SelectedCard(); ok {
		card.Hints = nil
		window := float64(screen.Bounds().Dx())
		panel := city.RectAt(0, top, width, bottom-top)
		room := city.RectAt(width, top, window-width, bottom-top)
		dock := ui.DockCard(th, card, panel, room, g.sidebarCovers(window, float64(screen.Bounds().Dy())), g.faces.Measure)
		foot = dock.Foot
		if dock.Beside {
			g.cardOver = dock.Card
		} else {
			g.card(screen, dock.Card)
		}
		g.pinned = dock.Card
		g.pinnedKinds = g.scene.Actions()
	}
	selected := ""
	if b := g.scene.Selected(); b != nil {
		selected = b.Session.ID
	}
	g.mu.Lock()
	skip, reveal := g.sidebarSkip, selected != g.sidebarFollowed
	g.sidebarFollowed = selected
	g.mu.Unlock()
	sb := ui.LayoutSidebar(th, top, foot, c.Projects(g.scene.Layout()), c.HiddenProjects(g.scene.Layout()), selected, skip, reveal, g.faces.Measure)
	for _, mark := range []ui.Text{sb.Above, sb.Below} {
		if mark.Text != "" {
			g.run(screen, mark, th.Palette.Dim)
		}
	}
	for _, row := range sb.Rows {
		if row.Selected {
			g.roundRect(screen, row.Rect.Inset(th.Grid()/2), th.Radius()/2, th.Palette.Hairline)
		}
		if row.Kind == ui.RowSession {
			centre := row.Dot.Center()
			g.fillCircle(screen, float32(centre.X), float32(centre.Y), float32(row.Dot.Width()/2), th.Color(row.Tone))
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
	g.sidebarSkip = sb.Skip
	g.mu.Unlock()
}

// sidebarCovers is what a card standing beside the sidebar keeps clear
// of: the lens panel, the stream and the minimap, where they are drawn.
func (g *Game) sidebarCovers(width, height float64) []city.Rect {
	covers := []city.Rect{g.streamLaid.Rect, g.minimapBox}
	if g.lensPanelShown() {
		covers = append(covers, ui.LayoutLensPanel(g.theme, g.scene.LensPanel(), g.lensArea(width, height, g.faces.Measure), g.faces.Measure).Rect)
	}
	return covers
}

// wheelSidebar scrolls the sidebar by whole rows when the wheel turns
// over it, so the map behind it does not zoom.
func (g *Game) wheelSidebar(cursor city.Point, notches float64) bool {
	if !g.sidebar || g.hidden {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.sidebarLayout.Rect.Contains(cursor) {
		return false
	}
	g.sidebarWheel += notches
	whole := math.Trunc(g.sidebarWheel)
	g.sidebarWheel -= whole
	g.sidebarSkip = min(max(g.sidebarSkip-int(whole), 0), g.sidebarLayout.Max)
	return true
}

// pageSidebar answers PageUp, PageDown, Home and End while the sidebar
// is open; the arrows stay with the map. The next layout keeps the
// scroll in range, so a key pressed before the panel is first drawn
// still counts.
func (g *Game) pageSidebar() bool {
	if !g.sidebar || g.hidden {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	page := max(g.sidebarLayout.Page, 1)
	switch {
	case g.just(ebiten.KeyPageDown):
		g.sidebarSkip += page
	case g.just(ebiten.KeyPageUp):
		g.sidebarSkip = max(g.sidebarSkip-page, 0)
	case g.just(ebiten.KeyHome):
		g.sidebarSkip = 0
	case g.just(ebiten.KeyEnd):
		g.sidebarSkip = math.MaxInt32
	default:
		return false
	}
	return true
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
