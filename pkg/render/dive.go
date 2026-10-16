package render

import (
	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/format"
	"botropolis.city/botropolis/pkg/ui"
)

// The dive is a focused group's stations listed beside the map, in line
// order, on the side away from the river (item 90). It is not on the
// overlay stack: the map behind it keeps every key but the dive's own.

// diveWidth is how wide the dive's list is, at most a third of the
// window.
func diveWidth(th ui.Theme, width float64) float64 {
	return min(th.Grid()*56, width/3)
}

// layoutDive reserves the dive's room on the scene, on the side away
// from the river, and is where its list stands while a group is
// focused, or nothing.
func (g *Game) layoutDive(width, height float64, measure ui.Measure) city.Rect {
	th := g.theme
	grid := th.Grid()
	side := g.scene.DiveSide()
	w := diveWidth(th, width)
	g.scene.SetDiveRoom(side, w+grid*2)
	rows := g.scene.Dive()
	if len(rows) == 0 {
		return city.Rect{}
	}
	in := g.scene.Insets()
	_, title := measure("", ui.Title)
	_, body := measure("", ui.Body)
	_, small := measure("", ui.Small)
	top := g.scene.TopChrome() + grid
	left := grid
	if g.sidebar {
		left += ui.SidebarWidth(th)
	}
	x := left
	if side == city.SideRight {
		x = width - grid - w
	}
	h := min(grid*3+title+small+float64(len(rows))*(body+small*2+grid), height-top-in.Bottom-grid)
	return city.RectAt(x, top, w, h)
}

// drawDive draws the dive's list, and is where it stands.
func (g *Game) drawDive(screen *ebiten.Image, width, height float64) city.Rect {
	dock := g.layoutDive(width, height, g.faces.Measure)
	if dock.Area() == 0 {
		return dock
	}
	th := g.theme
	grid := th.Grid()
	_, title := g.faces.Measure("", ui.Title)
	_, body := g.faces.Measure("", ui.Body)
	_, small := g.faces.Measure("", ui.Small)
	g.roundPanel(screen, dock)
	inner := dock.Width() - grid*2
	x, y := dock.Min.X+grid, dock.Min.Y+grid
	name := ""
	for _, gr := range g.scene.Groups() {
		if gr.ID == g.scene.FocusedGroup() {
			name = gr.Name
		}
	}
	g.text(screen, city.Point{X: x, Y: y}, g.clip(name, inner, ui.Title), ui.Title, th.Palette.Text)
	y += title
	g.text(screen, city.Point{X: x, Y: y}, g.clip("j k step   enter reads   esc leaves", inner, ui.Small), ui.Small, th.Palette.Dim)
	y += small + grid
	rowHeight := body + small*2 + grid
	for i, row := range g.scene.Dive() {
		if y+rowHeight > dock.Max.Y {
			break
		}
		if i == g.scene.DiveStep() {
			g.roundRect(screen, city.RectAt(dock.Min.X+grid/2, y-grid/4, dock.Width()-grid, rowHeight-grid/2), 0, th.Palette.Hairline)
		}
		site, move := diveColumns(row, inner, g.faces.Measure)
		mw, _ := g.faces.Measure(move, ui.Small)
		g.text(screen, city.Point{X: x, Y: y}, site, ui.Body, th.Palette.Text)
		g.text(screen, city.Point{X: x + inner - mw, Y: y}, move, ui.Small, th.Color(ui.MoveTone(row.Move)))
		g.text(screen, city.Point{X: x, Y: y + body}, g.clip(row.Sentence, inner, ui.Small), ui.Small, th.Palette.Dim)
		if row.PR != "" {
			g.text(screen, city.Point{X: x, Y: y + body + small}, g.clip(row.PR, inner, ui.Small), ui.Small, th.Palette.Dim)
		}
		y += rowHeight
	}
	return dock
}

// diveColumns are a row's two heads: the site on the left, and whose
// move it is with how long it has waited on the right. The time is
// never cut: whose move gives way first, then the site.
func diveColumns(row city.DiveRow, inner float64, measure ui.Measure) (site, move string) {
	waited := "waited " + format.Age(row.Waited)
	ww, _ := measure(waited, ui.Small)
	gap, _ := measure("   ", ui.Small)
	move = waited
	if who := clipTo(string(row.Move), inner*0.7-ww-gap, ui.Small, measure); who != "" {
		move = who + "   " + waited
	}
	mw, _ := measure(move, ui.Small)
	return clipTo(row.Site, inner-mw-gap, ui.Body, measure), move
}

// diveKeys answers j, k, Enter and Esc while a group is focused, and
// reports whether it took the key.
func (g *Game) diveKeys() bool {
	if g.scene.FocusedGroup() == "" {
		return false
	}
	switch {
	case g.just(ebiten.KeyJ):
		g.scene.StepDive(1)
	case g.just(ebiten.KeyK):
		g.scene.StepDive(-1)
	case (g.just(ebiten.KeyEnter) || g.just(ebiten.KeyKPEnter)) && g.scene.DiveStep() >= 0:
		g.openReader()
	case g.just(ebiten.KeyEscape):
		g.scene.LeaveDive()
		g.scene.Fit()
	default:
		return false
	}
	return true
}
