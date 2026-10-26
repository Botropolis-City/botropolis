package render

import (
	"image/color"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// closeUp is the Close up panel (item 86, slice 4). Closing up records
// the desk with the daemon: when, what was open, what you pinned to start
// with next time. Each live session is left running unless you mark it
// pause, which is claude stop and keeps the conversation; a session
// mid-turn says pausing interrupts it, and one in a terminal cannot be
// paused from here. It is optional: Shift+Q opens it, or the button
// beside the quit prompt, and quitting without it changes nothing. It is
// a way of quitting: once it has recorded and paused, the city leaves
// (item 291).
type closeUp struct {
	rows     []state.Session
	pinned   map[string]bool
	paused   map[string]bool
	selected int
	// err is why the last close-up failed, shown on the panel, which
	// stays open so nothing looks done that was not.
	err string
}

const closeUpLabel = "close up"

// openCloseUp puts the panel on the stack, standing the quit prompt down.
// With nothing live there is nothing to choose, so it records the
// close-up at once.
func (g *Game) openCloseUp() {
	if g.quit.armed(timeNow()) {
		g.quit.cancel()
		g.clearPrompt()
	}
	rows := g.scene.OpenSessions()
	if len(rows) == 0 {
		if err := g.closeUpWith(city.Action{Kind: city.ActionCloseUp}); err != nil {
			g.SetStatus(err.Error())
		}
		return
	}
	g.overlays.push(&closeUp{rows: rows, pinned: map[string]bool{}, paused: map[string]bool{}})
}

// closeUpWith has the actor record and pause, and marks the city closed
// up only when both did: a failure quits nothing.
func (g *Game) closeUpWith(action city.Action) error {
	if g.actor == nil {
		return nil
	}
	if err := g.actor.Do(action); err != nil {
		return err
	}
	g.closedUp.Store(true)
	return nil
}

func (p *closeUp) ids() []string {
	out := make([]string, 0, len(p.rows))
	for _, s := range p.rows {
		out = append(out, s.ID)
	}
	return out
}

func (p *closeUp) marked(set map[string]bool) []string {
	var out []string
	for _, s := range p.rows {
		if set[s.ID] {
			out = append(out, s.ID)
		}
	}
	return out
}

func (p *closeUp) pinnedIDs() []string { return p.marked(p.pinned) }
func (p *closeUp) pausedIDs() []string { return p.marked(p.paused) }

// pausable is whether a session can be paused from here: one open in a
// terminal cannot.
func pausable(s state.Session) bool { return s.Kind == claude.KindBackground }

// line is a row's words: its title and project, and why it cannot be
// paused or what pausing it would do. Whether it is marked pause or
// pinned is drawn as its box and its pin mark.
func (p *closeUp) line(i int) string {
	s := p.rows[i]
	title := s.Title
	if title == "" {
		title = state.ShortID(s.ID)
	}
	parts := []string{title, filepath.Base(s.CWD)}
	switch {
	case !pausable(s):
		parts = append(parts, "runs in a terminal — stays running")
	case s.State == state.Working:
		parts = append(parts, "working — pausing interrupts it")
	}
	return strings.Join(parts, "  ·  ")
}

// closeUpRow is a row as laid out: its box for pause, when it can be
// paused, its pin mark, and its words.
type closeUpRow struct {
	Rect, Box, Pin  city.Rect
	HasBox, Checked bool
	Pinned          bool
	Text            ui.Text
}

// metrics are the grid and a line's height, measured when there are
// faces and estimated when there are none, as in tests.
func (g *Game) metrics() (float64, float64, ui.Measure) {
	if g.faces == nil {
		return 8, 16, func(s string, _ ui.Size) (float64, float64) { return 7 * float64(len([]rune(s))), 16 }
	}
	_, lineH := g.faces.Measure("", ui.Body)
	return g.theme.Grid(), lineH, g.faces.Measure
}

// plate is the panel for a window.
func (p *closeUp) plate(g *Game, width, height float64) city.Rect {
	grid, lineH, _ := g.metrics()
	w := min(width-8*grid, 760)
	h := float64(len(p.rows)+6)*(lineH+grid) + 2*grid
	if p.err != "" {
		h += lineH
	}
	return city.RectAt((width-w)/2, (height-h)/2, w, h)
}

// layout lays the rows out: a box for pause at the left, then the words,
// then the pin mark at the right.
func (p *closeUp) layout(g *Game, width, height float64) []closeUpRow {
	grid, lineH, measure := g.metrics()
	plate := p.plate(g, width, height)
	x, y := plate.Min.X+2*grid, plate.Min.Y+2*grid+4*lineH+4*grid
	box := lineH * 0.8
	pinW, _ := measure("pin", ui.Small)
	pinW = max(pinW, box)
	rows := make([]closeUpRow, 0, len(p.rows))
	for i, s := range p.rows {
		r := closeUpRow{
			Rect:    city.RectAt(plate.Min.X+grid, y-grid/2, plate.Width()-2*grid, lineH+grid),
			Box:     city.RectAt(x, y+(lineH-box)/2, box, box),
			HasBox:  pausable(s),
			Checked: p.paused[s.ID],
			Pin:     city.RectAt(plate.Max.X-2*grid-pinW, y, pinW, lineH),
			Pinned:  p.pinned[s.ID],
		}
		textX := x + box + grid
		r.Text = ui.Text{Text: ui.Fit(p.line(i), r.Pin.Min.X-grid-textX, ui.Body, measure), At: city.Point{X: textX, Y: y}, Size: ui.Body}
		rows = append(rows, r)
		y += lineH + grid
	}
	return rows
}

func (p *closeUp) keys(g *Game) bool {
	switch {
	case g.just(ebiten.KeyEscape):
		return false
	case g.just(ebiten.KeyArrowDown) || g.just(ebiten.KeyJ):
		p.selected = min(len(p.rows)-1, p.selected+1)
	case g.just(ebiten.KeyArrowUp) || g.just(ebiten.KeyK):
		p.selected = max(0, p.selected-1)
	case g.just(ebiten.KeySpace):
		id := p.rows[p.selected].ID
		p.pinned[id] = !p.pinned[id]
	case g.just(ebiten.KeyP):
		if s := p.rows[p.selected]; pausable(s) {
			p.paused[s.ID] = !p.paused[s.ID]
		}
	case g.just(ebiten.KeyA):
		for _, s := range p.rows {
			p.paused[s.ID] = pausable(s)
		}
	case g.just(ebiten.KeyN):
		clear(p.paused)
	case g.just(ebiten.KeyEnter) || g.just(ebiten.KeyKPEnter):
		if g.actor == nil {
			return false
		}
		if err := g.closeUpWith(city.Action{Kind: city.ActionCloseUp, Open: p.ids(), Pinned: p.pinnedIDs(), Pause: p.pausedIDs()}); err != nil {
			p.err = err.Error()
			return true
		}
		return false
	}
	return true
}

// click toggles the pause box or the pin mark under the pointer.
func (p *closeUp) click(g *Game, at city.Point) bool {
	size := g.scene.Size()
	for i, r := range p.layout(g, size.X, size.Y) {
		id := p.rows[i].ID
		switch {
		case r.HasBox && r.Box.Contains(at):
			p.paused[id] = !p.paused[id]
			p.selected = i
		case r.Pin.Contains(at):
			p.pinned[id] = !p.pinned[id]
			p.selected = i
		}
	}
	return true
}

func (p *closeUp) draw(g *Game, screen *ebiten.Image, width, height float64) {
	th := g.theme
	scrim := th.Palette.Panel
	scrim.A = 0x60
	g.roundRect(screen, city.RectAt(0, 0, width, height), 0, scrim)
	grid, lineH, _ := g.metrics()
	plate := p.plate(g, width, height)
	g.roundRect(screen, plate, th.Radius(), th.Palette.Panel)
	x, y := plate.Min.X+2*grid, plate.Min.Y+2*grid
	g.text(screen, city.Point{X: x, Y: y}, "Close up", ui.Body, th.Palette.Text)
	y += lineH + grid
	room := plate.Width() - 4*grid
	g.text(screen, city.Point{X: x, Y: y}, ui.Fit(city.ActionCloseUp.Describe(), room, ui.Small, g.faces.Measure), ui.Small, th.Palette.Dim)
	y += lineH
	g.text(screen, city.Point{X: x, Y: y}, ui.Fit("pause stops a session for now and keeps its conversation; pin marks what to start with next time", room, ui.Small, g.faces.Measure), ui.Small, th.Palette.Dim)
	rows := p.layout(g, width, height)
	for i, r := range rows {
		if i == p.selected {
			g.roundRect(screen, r.Rect, 0, th.Palette.Hairline)
		}
		if r.HasBox {
			g.checkbox(screen, r.Box, r.Checked)
		}
		g.text(screen, r.Text.At, r.Text.Text, r.Text.Size, th.Palette.Text)
		mark := r.Pin.Center()
		radius := float32(r.Box.Width() / 2)
		if r.Pinned {
			g.fillCircle(screen, float32(mark.X), float32(mark.Y), radius, th.Palette.Accent)
		} else {
			g.strokeCircle(screen, float32(mark.X), float32(mark.Y), radius, 1.5, th.Palette.Dim)
		}
	}
	if len(rows) > 0 {
		head := rows[0].Rect.Min.Y - lineH - grid/2
		g.text(screen, city.Point{X: rows[0].Box.Min.X, Y: head}, "pause", ui.Small, th.Palette.Dim)
		pinW, _ := g.faces.Measure("pin", ui.Small)
		g.text(screen, city.Point{X: rows[0].Pin.Center().X - pinW/2, Y: head}, "pin", ui.Small, th.Palette.Dim)
	}
	y = plate.Max.Y - lineH - 2*grid
	if p.err != "" {
		g.text(screen, city.Point{X: x, Y: y - lineH}, p.err, ui.Small, th.Palette.NeedsYou)
	}
	g.text(screen, city.Point{X: x, Y: y}, "click a box, or p, to pause  ·  a all  ·  n none  ·  space or click pin  ·  enter close up and quit  ·  esc back", ui.Small, th.Palette.Dim)
}

// checkbox is a box, ticked when checked.
func (g *Game) checkbox(screen *ebiten.Image, box city.Rect, checked bool) {
	th := g.theme
	if checked {
		g.roundRect(screen, box, th.Radius()/3, th.Palette.Accent)
		tick := []city.Point{
			{X: box.Min.X + box.Width()*0.22, Y: box.Min.Y + box.Height()*0.52},
			{X: box.Min.X + box.Width()*0.42, Y: box.Min.Y + box.Height()*0.72},
			{X: box.Min.X + box.Width()*0.78, Y: box.Min.Y + box.Height()*0.3},
		}
		for i := 1; i < len(tick); i++ {
			g.strokeLine(screen, float32(tick[i-1].X), float32(tick[i-1].Y), float32(tick[i].X), float32(tick[i].Y), float32(max(1.5, box.Width()/7)), th.Palette.Panel)
		}
		return
	}
	g.strokeRound(screen, box, th.Palette.Dim)
}

// strokeRound outlines a rect.
func (g *Game) strokeRound(screen *ebiten.Image, r city.Rect, c color.NRGBA) {
	g.strokeRect(screen, float32(r.Min.X), float32(r.Min.Y), float32(r.Width()), float32(r.Height()), 1.5, c)
}

// quitButton is the close up button beside the quit prompt, while the
// prompt is up: closing up stays one choice away from quitting.
func (g *Game) quitButton() (ui.Button, bool) {
	g.mu.Lock()
	prompt := g.notice.Text(timeNow()) == quitPrompt
	lines := g.footerBar.Lines
	g.mu.Unlock()
	if !prompt {
		return ui.Button{}, false
	}
	measure := g.measure
	if g.faces == nil {
		measure = func(s string) (float64, float64) { return 7 * float64(len(s)), 14 }
	}
	at := city.Point{}
	if len(lines) > 0 {
		last := lines[len(lines)-1]
		w, _ := measure(last.Text)
		at = city.Point{X: last.At.X + w + 16, Y: last.At.Y - 4}
	}
	w, h := measure(closeUpLabel)
	return ui.Button{Rect: city.RectAt(at.X, at.Y, w+16, h+8), Label: ui.Text{Text: closeUpLabel, At: city.Point{X: at.X + 8, Y: at.Y + 4}, Size: ui.Small}}, true
}
