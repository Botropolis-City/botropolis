package render

import (
	"fmt"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// clinic is the clinic's menu, a panel on the overlay stack: what needs
// you, then what is in trouble, one row each, and 1 to 9 or enter jumps
// to the session (item 280).
type clinic struct {
	selected int
	rect     city.Rect
	// filter narrows the rows while / is on (item 350).
	filter listFilter
}

// clinicLabels are what a filter matches a clinic row by: its title and
// why it is listed.
func clinicLabels(g *Game, rows []city.ClinicRow) []string {
	labels := make([]string, len(rows))
	for i, r := range rows {
		labels[i] = r.Building.Card(g.now()).Title + " " + r.Reason
	}
	return labels
}

// shownRows is the rows the filter leaves, and where the selection is
// among them.
func (c *clinic) shownRows(g *Game) ([]city.ClinicRow, int) {
	all := g.scene.Clinic()
	var rows []city.ClinicRow
	selected := 0
	for _, i := range c.filter.visible(clinicLabels(g, all)) {
		if i == c.selected {
			selected = len(rows)
		}
		rows = append(rows, all[i])
	}
	return rows, selected
}

// clinicDigits are the keys that jump to the first nine rows.
var clinicDigits = []ebiten.Key{
	ebiten.KeyDigit1, ebiten.KeyDigit2, ebiten.KeyDigit3, ebiten.KeyDigit4, ebiten.KeyDigit5,
	ebiten.KeyDigit6, ebiten.KeyDigit7, ebiten.KeyDigit8, ebiten.KeyDigit9,
}

func (g *Game) clinicMenu() *clinic {
	c, _ := g.overlays.top().(*clinic)
	return c
}

func (g *Game) openClinic() {
	if g.clinicMenu() != nil {
		return
	}
	g.overlays.push(&clinic{})
}

// clinicStatus is the menu's line under its title.
func clinicStatus(count int) string {
	if count == 0 {
		return "nobody needs you and nothing is in trouble"
	}
	return fmt.Sprintf("%d to see to   1-9 or ↑ ↓ enter jumps to it, esc closes", count)
}

// status is the line under the title. It counts every row, filtered
// or not: a filter that matches nothing does not mean nobody needs you.
func (c *clinic) status(g *Game) string {
	return clinicStatus(len(g.scene.Clinic()))
}

// clinicPlate is the clinic's name with its count, and the tone it
// takes: the needs-you colour while it counts anyone.
func clinicPlate(count int) (string, ui.Tone) {
	if count == 0 {
		return "clinic", ui.ToneNone
	}
	return "clinic " + strconv.Itoa(count), ui.ToneNeedsYou
}

func (c *clinic) keys(g *Game) bool {
	rows := g.scene.Clinic()
	var starts []int
	for i, r := range rows {
		if i == 0 || r.Section != rows[i-1].Section {
			starts = append(starts, i)
		}
	}
	switch g.listKeys(&c.selected, len(rows), listShape{starts: starts, labels: clinicLabels(g, rows), filter: &c.filter}) {
	case listBack:
		return false
	case listChoose:
		return c.jump(g, rows[c.selected])
	}
	return true
}

// jump selects a row's session, which centres the map on it, and
// closes the menu.
func (c *clinic) jump(g *Game, row city.ClinicRow) bool {
	g.scene.Select(row.Building.Session.ID)
	return false
}

// click closes the menu when the press lands outside it.
func (c *clinic) click(g *Game, at city.Point) bool {
	g.mu.Lock()
	panel := c.rect
	g.mu.Unlock()
	return panel.Contains(at)
}

func (c *clinic) draw(g *Game, screen *ebiten.Image, width, height float64) {
	th := g.theme
	scrim := th.Palette.Panel
	scrim.A = 0x60
	g.roundRect(screen, city.RectAt(0, 0, width, height), 0, scrim)
	rows, selected := c.shownRows(g)
	grid := th.Grid()
	_, title := g.faces.Measure("", ui.Title)
	_, body := g.faces.Measure("", ui.Body)
	_, small := g.faces.Measure("", ui.Small)
	rowHeight := body + grid
	heading := small + grid/2
	panelWidth := min(width-grid*8, grid*100)
	shown := min(len(rows), max(1, int((height-grid*16-title-small-2*heading)/rowHeight)))
	sections := 0
	for i := range shown {
		if i == 0 || rows[i].Section != rows[i-1].Section {
			sections++
		}
	}
	filterH := 0.0
	if c.filter.on {
		filterH = small + grid/2
	}
	panel := city.RectAt((width-panelWidth)/2, grid*6, panelWidth,
		grid*4+title+small+grid+filterH+float64(sections)*heading+float64(shown)*rowHeight)
	g.mu.Lock()
	c.rect = panel
	g.mu.Unlock()
	g.roundRect(screen, panel, th.Radius(), th.Palette.Panel)
	inner := panelWidth - grid*4
	x, y := panel.Min.X+grid*2, panel.Min.Y+grid*2
	g.text(screen, city.Point{X: x, Y: y}, "Clinic · needs you, then in trouble", ui.Title, th.Palette.Text)
	y += title
	g.text(screen, city.Point{X: x, Y: y}, c.status(g), ui.Small, th.Palette.Dim)
	y += small + grid
	y += g.drawFilter(screen, &c.filter, city.Point{X: x, Y: y}, ui.Small)
	first := max(0, min(selected-shown+1, len(rows)-shown))
	for i := first; i < first+shown && i < len(rows); i++ {
		row := rows[i]
		tone := ui.ToneNeedsYou
		if row.Section == city.ClinicInTrouble {
			tone = ui.ToneError
		}
		if i == first || row.Section != rows[i-1].Section {
			g.text(screen, city.Point{X: x, Y: y}, string(row.Section), ui.Small, th.Color(tone))
			y += heading
		}
		band := city.RectAt(panel.Min.X+grid, y-grid/4, panelWidth-grid*2, rowHeight-grid/2)
		g.rowShown(c, i, band)
		if i == selected {
			g.roundRect(screen, band, 0, th.Palette.Hairline)
		}
		key := "  "
		if i < len(clinicDigits) {
			key = strconv.Itoa(i + 1)
		}
		g.text(screen, city.Point{X: x, Y: y}, key, ui.Body, th.Palette.Accent)
		name := g.clip(row.Building.Card(g.now()).Title, inner*0.45, ui.Body)
		g.text(screen, city.Point{X: x + grid*3, Y: y}, name, ui.Body, th.Palette.Text)
		reason := g.clip(row.Reason, inner*0.5, ui.Small)
		rw, _ := g.faces.Measure(reason, ui.Small)
		g.text(screen, city.Point{X: x + inner - rw, Y: y + (body-small)/2}, reason, ui.Small, th.Color(tone))
		y += rowHeight
	}
}
