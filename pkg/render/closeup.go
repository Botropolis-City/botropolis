package render

import (
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
// beside the quit prompt, and quitting without it changes nothing.
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
		g.act(city.Action{Kind: city.ActionCloseUp})
		return
	}
	g.overlays.push(&closeUp{rows: rows, pinned: map[string]bool{}, paused: map[string]bool{}})
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

// line is a row as the panel shows it.
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
	case p.paused[s.ID] && s.State == state.Working:
		parts = append(parts, "pause", "working — pausing interrupts it")
	case p.paused[s.ID]:
		parts = append(parts, "pause")
	case s.State == state.Working:
		parts = append(parts, "leave running", "working — pausing interrupts it")
	default:
		parts = append(parts, "leave running")
	}
	if p.pinned[s.ID] {
		parts = append(parts, "pinned for next time")
	}
	return strings.Join(parts, "  ·  ")
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
		if err := g.actor.Do(city.Action{Kind: city.ActionCloseUp, Open: p.ids(), Pinned: p.pinnedIDs(), Pause: p.pausedIDs()}); err != nil {
			p.err = err.Error()
			return true
		}
		return false
	}
	return true
}

func (p *closeUp) click(g *Game, at city.Point) bool { return true }

func (p *closeUp) draw(g *Game, screen *ebiten.Image, width, height float64) {
	th := g.theme
	scrim := th.Palette.Panel
	scrim.A = 0x60
	g.roundRect(screen, city.RectAt(0, 0, width, height), 0, scrim)
	grid := th.Grid()
	_, lineH := g.faces.Measure("", ui.Body)
	w := min(width-8*grid, 760)
	h := float64(len(p.rows)+7) * (lineH + grid)
	plate := city.RectAt((width-w)/2, (height-h)/2, w, h)
	g.roundRect(screen, plate, th.Radius(), th.Palette.Panel)
	x, y := plate.Min.X+2*grid, plate.Min.Y+2*grid
	g.text(screen, city.Point{X: x, Y: y}, "Close up", ui.Body, th.Palette.Text)
	y += lineH + grid
	g.text(screen, city.Point{X: x, Y: y}, city.ActionCloseUp.Describe(), ui.Small, th.Palette.Dim)
	y += lineH
	g.text(screen, city.Point{X: x, Y: y}, "pause keeps the conversation; you can resume it", ui.Small, th.Palette.Dim)
	y += lineH + 2*grid
	for i := range p.rows {
		if i == p.selected {
			g.roundRect(screen, city.RectAt(plate.Min.X+grid, y-grid/2, w-2*grid, lineH+grid), 0, th.Palette.Hairline)
		}
		g.text(screen, city.Point{X: x, Y: y}, p.line(i), ui.Body, th.Palette.Text)
		y += lineH + grid
	}
	y += grid
	if p.err != "" {
		g.text(screen, city.Point{X: x, Y: y}, p.err, ui.Small, th.Palette.NeedsYou)
		y += lineH
	}
	g.text(screen, city.Point{X: x, Y: y}, "p pause or leave running  ·  a pause all  ·  n leave all running  ·  space pin for next time  ·  enter close up  ·  esc leave", ui.Small, th.Palette.Dim)
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
