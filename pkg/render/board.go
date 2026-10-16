package render

import (
	"fmt"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/format"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// board is City Hall's board of groups, a panel on the overlay stack:
// one row per group, and enter focuses the map on the one chosen
// (item 87).
type board struct {
	selected int
	rect     city.Rect
	// newButton is New session's button on the board.
	newButton city.Rect
}

type boardRow struct {
	Name     string
	Move     string
	Waited   string
	LastTime string
	Next     string
}

func boardRows(groups []state.Group, now time.Time) []boardRow {
	rows := make([]boardRow, 0, len(groups))
	for _, g := range groups {
		rows = append(rows, boardRow{
			Name:     g.Name,
			Move:     string(g.Move) + ": " + g.Why,
			Waited:   format.Age(now.Sub(g.Journal.Active)),
			LastTime: g.Journal.LastTime,
			Next:     g.Journal.Next,
		})
	}
	return rows
}

func boardStatus(rows []boardRow) string {
	if len(rows) == 0 {
		return "no groups yet: sessions join one when they share a ticket, a link or their wording"
	}
	return fmt.Sprintf("%d groups   ↑ ↓ choose, enter focuses it on the map, esc closes", len(rows))
}

func (g *Game) board() *board {
	b, _ := g.overlays.top().(*board)
	return b
}

func (g *Game) openBoard() {
	if g.board() != nil {
		return
	}
	g.overlays.push(&board{})
}

// clickLandmark is what a click on a landmark opens beside selecting it:
// the plant its breakdown, City Hall its board.
func (g *Game) clickLandmark(l city.Landmark) {
	switch l {
	case city.LandmarkPlant:
		g.breakdown, g.window = true, city.LastDay
	case city.LandmarkHall:
		g.openBoard()
	}
}

func (b *board) keys(g *Game) bool {
	groups := g.scene.Groups()
	switch {
	case g.just(ebiten.KeyEscape):
		return false
	case g.just(ebiten.KeyEnter), g.just(ebiten.KeyKPEnter):
		if b.selected < len(groups) {
			g.scene.FocusGroup(groups[b.selected].ID)
		}
		return false
	case g.just(ebiten.KeyArrowDown):
		b.selected = max(0, min(b.selected+1, len(groups)-1))
	case g.just(ebiten.KeyArrowUp):
		b.selected = max(0, b.selected-1)
	case g.just(ebiten.KeyR):
		// The first run takes the board's place on the stack, so the
		// board stays closed behind it.
		g.overlays.pop()
		if !g.openFirstRun() {
			g.overlays.push(b)
		}
	}
	return true
}

// click closes the board when the press lands outside it.
func (b *board) click(g *Game, at city.Point) bool {
	g.mu.Lock()
	panel, button := b.rect, b.newButton
	g.mu.Unlock()
	if button.Contains(at) {
		g.openNewSession()
		return true
	}
	return panel.Contains(at)
}

func (b *board) draw(g *Game, screen *ebiten.Image, width, height float64) {
	th := g.theme
	scrim := th.Palette.Panel
	scrim.A = 0x60
	g.roundRect(screen, city.RectAt(0, 0, width, height), 0, scrim)
	rows := boardRows(g.scene.Groups(), g.now())
	grid := th.Grid()
	_, title := g.faces.Measure("", ui.Title)
	_, body := g.faces.Measure("", ui.Body)
	_, small := g.faces.Measure("", ui.Small)
	rowHeight := body + small*2 + grid
	panelWidth := min(width-grid*8, grid*120)
	shown := min(len(rows), max(1, int((height-grid*16-title-small)/rowHeight)))
	panel := city.RectAt((width-panelWidth)/2, grid*6, panelWidth, grid*4+title+small+grid+float64(shown)*rowHeight)
	g.mu.Lock()
	b.rect = panel
	g.mu.Unlock()
	g.roundRect(screen, panel, th.Radius(), th.Palette.Panel)
	inner := panelWidth - grid*4
	x, y := panel.Min.X+grid*2, panel.Min.Y+grid*2
	g.text(screen, city.Point{X: x, Y: y}, "City Hall · groups", ui.Title, th.Palette.Text)
	label := "New session  " + g.findChords().New.String()
	lw, lh := g.faces.Measure(label, ui.Small)
	button := city.RectAt(panel.Max.X-grid*2-lw-grid*2, y, lw+grid*2, lh+grid/2)
	g.mu.Lock()
	b.newButton = button
	g.mu.Unlock()
	g.roundRect(screen, button, th.Radius()/2, th.Palette.Hairline)
	g.text(screen, city.Point{X: button.Min.X + grid, Y: button.Min.Y + grid/4}, label, ui.Small, th.Palette.Accent)
	y += title
	status := boardStatus(rows)
	if len(rows) > 0 && !g.scene.Snapshot().GroupsReviewed {
		status += ", r reviews what was found"
	}
	g.text(screen, city.Point{X: x, Y: y}, status, ui.Small, th.Palette.Dim)
	y += small + grid
	first := max(0, min(b.selected-shown+1, len(rows)-shown))
	for i := first; i < first+shown && i < len(rows); i++ {
		row := rows[i]
		if i == b.selected {
			g.roundRect(screen, city.RectAt(panel.Min.X+grid, y-grid/4, panelWidth-grid*2, rowHeight-grid/2), 0, th.Palette.Hairline)
		}
		head := g.clip(row.Name, inner*0.45, ui.Body)
		g.text(screen, city.Point{X: x, Y: y}, head, ui.Body, th.Palette.Text)
		move := g.clip(row.Move+"   waited "+row.Waited, inner*0.53, ui.Small)
		mw, _ := g.faces.Measure(move, ui.Small)
		g.text(screen, city.Point{X: x + inner - mw, Y: y}, move, ui.Small, th.Color(ui.MoveTone(g.scene.Groups()[i].Move)))
		g.text(screen, city.Point{X: x, Y: y + body}, g.clip("last time  "+row.LastTime, inner, ui.Small), ui.Small, th.Palette.Dim)
		g.text(screen, city.Point{X: x, Y: y + body + small}, g.clip("next       "+row.Next, inner, ui.Small), ui.Small, th.Palette.Dim)
		y += rowHeight
	}
}

// clip cuts text to a width, ending it with an ellipsis when it is cut.
func (g *Game) clip(s string, width float64, size ui.Size) string {
	if w, _ := g.faces.Measure(s, size); w <= width {
		return s
	}
	r := []rune(s)
	for len(r) > 0 {
		r = r[:len(r)-1]
		if w, _ := g.faces.Measure(string(r)+"…", size); w <= width {
			return string(r) + "…"
		}
	}
	return ""
}
