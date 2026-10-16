package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// journal is the panel the map opens on after a gap (item 86, slice 5):
// what you were doing and what is next, one entry per group and per
// session in no group, under a strip of what changed since you left,
// with Restore the desk beside it. Esc goes to the map, and the journal
// key brings it back.
type journal struct {
	entries  []city.JournalEntry
	since    time.Time
	selected int
	// note is what the last restore did, or why it did nothing.
	note string
}

const restoreLabel = "Restore the desk"

// closeUpFresh is how long before the map was last seen a close-up may
// have been taken and still be the desk the map left: one taken days
// earlier is not.
const closeUpFresh = time.Hour

// maybeJournal opens the journal once, on the first snapshot, when the
// map was last seen journal_hours or more ago. It needs no input, so a
// city opened after a gap shows where you were on its first frame.
func (g *Game) maybeJournal() {
	if g.journalChecked {
		return
	}
	g.journalChecked = true
	if g.awaySince.IsZero() || g.journalHours <= 0 {
		return
	}
	if g.scene.Now().Sub(g.awaySince) < time.Duration(g.journalHours)*time.Hour {
		return
	}
	g.openJournal()
}

// openJournal puts the journal on the stack, read now from what Claude
// already wrote.
func (g *Game) openJournal() {
	if _, open := g.overlays.top().(*journal); open {
		return
	}
	closed, _ := g.closeUp()
	g.overlays.push(&journal{entries: city.BuildJournal(g.scene.Snapshot(), closed.Pinned, g.replyReader()), since: g.awaySince})
}

// closeUp is the last close-up, when it is the one the map left on.
func (g *Game) closeUp() (state.CloseUp, bool) {
	if g.lastCloseUp == nil {
		return state.CloseUp{}, false
	}
	c, err := g.lastCloseUp()
	if err != nil || c.At.IsZero() || g.awaySince.Sub(c.At) > closeUpFresh {
		return state.CloseUp{}, false
	}
	return c, true
}

func (j *journal) titles() []string {
	out := make([]string, 0, len(j.entries))
	for _, e := range j.entries {
		out = append(out, e.Title)
	}
	return out
}

// restore reopens in the background each session open when you left and
// not live now: the ones your close-up noted, or, without one, the ones
// the map last saw live.
func (j *journal) restore(g *Game) {
	desk := g.scene.Layout().LastLive
	if c, ok := g.closeUp(); ok {
		desk = c.Open
	}
	live := map[string]bool{}
	known := map[string]bool{}
	for _, s := range g.scene.Snapshot().Sessions {
		known[s.ID] = true
		live[s.ID] = state.Live(s.State) || s.State == state.Empty
	}
	var reopen []string
	for _, id := range desk {
		if known[id] && !live[id] {
			reopen = append(reopen, id)
		}
	}
	if len(reopen) == 0 {
		j.note = "nothing was open to restore"
		return
	}
	g.act(city.Action{Kind: city.ActionRestore, Open: reopen})
	j.note = fmt.Sprintf("restoring %s in the background", plural(len(reopen), "session"))
}

func (j *journal) keys(g *Game) bool {
	switch {
	case g.just(ebiten.KeyEscape):
		return false
	case g.just(ebiten.KeyArrowDown) || g.just(ebiten.KeyJ):
		j.selected = min(len(j.entries)-1, j.selected+1)
	case g.just(ebiten.KeyArrowUp) || g.just(ebiten.KeyK):
		j.selected = max(0, j.selected-1)
	case g.just(ebiten.KeyR):
		j.restore(g)
	case g.just(ebiten.KeyEnter) || g.just(ebiten.KeyKPEnter):
		if len(j.entries) > 0 {
			g.scene.Select(j.entries[j.selected].Fly)
		}
		return false
	}
	return true
}

func (j *journal) click(g *Game, at city.Point) bool {
	size := g.scene.Size()
	if j.restoreButton(g, size.X, size.Y).Rect.Contains(at) {
		j.restore(g)
	}
	return true
}

// plate is the journal's panel for a window.
func (j *journal) plate(g *Game, width, height float64) (city.Rect, float64, float64) {
	grid, lineH := 8.0, 16.0
	if g.faces != nil {
		grid = g.theme.Grid()
		_, lineH = g.faces.Measure("", ui.Body)
	}
	w := min(width-8*grid, 820)
	h := min(height-8*grid, float64(4*len(j.entries)+6)*(lineH+grid))
	return city.RectAt((width-w)/2, (height-h)/2, w, h), grid, lineH
}

// restoreButton is Restore the desk, at the panel's top right, beside the
// journal.
func (j *journal) restoreButton(g *Game, width, height float64) ui.Button {
	plate, grid, lineH := j.plate(g, width, height)
	labelW := 7 * float64(len(restoreLabel))
	if g.faces != nil {
		labelW, _ = g.faces.Measure(restoreLabel, ui.Body)
	}
	rect := city.RectAt(plate.Max.X-2*grid-labelW-2*grid, plate.Min.Y+grid, labelW+2*grid, lineH+grid)
	return ui.Button{Rect: rect, Label: ui.Text{Text: restoreLabel, At: city.Point{X: rect.Min.X + grid, Y: rect.Min.Y + grid/2}, Size: ui.Body}}
}

func (j *journal) draw(g *Game, screen *ebiten.Image, width, height float64) {
	th := g.theme
	scrim := th.Palette.Panel
	scrim.A = 0x60
	g.roundRect(screen, city.RectAt(0, 0, width, height), 0, scrim)
	plate, grid, lineH := j.plate(g, width, height)
	g.roundRect(screen, plate, th.Radius(), th.Palette.Panel)
	button := j.restoreButton(g, width, height)
	g.button(screen, button.Rect, button.Label, th.Palette.Accent, button.Rect.Contains(g.pointer()))
	x, y := plate.Min.X+2*grid, plate.Min.Y+2*grid
	g.text(screen, city.Point{X: x, Y: y}, "Where you were", ui.Body, th.Palette.Text)
	y += lineH + grid
	g.text(screen, city.Point{X: x, Y: y}, sinceStrip(j.since, g.scene.Away(j.since), g.scene.Now().Location()), ui.Small, th.Palette.Dim)
	y += lineH + 2*grid
	for i, e := range j.entries {
		if y+3*lineH > plate.Max.Y-2*lineH {
			break
		}
		if i == j.selected {
			g.roundRect(screen, city.RectAt(plate.Min.X+grid, y-grid/2, plate.Width()-2*grid, 3*lineH+grid), 0, th.Palette.Hairline)
		}
		title := e.Title
		if e.Pinned {
			title += "  ·  pinned"
		}
		title += "  ·  " + strings.Join(e.Where, ", ")
		g.text(screen, city.Point{X: x, Y: y}, title, ui.Body, th.Palette.Text)
		g.text(screen, city.Point{X: x, Y: y + lineH}, "Last time: "+oneLine(e.LastTime), ui.Small, th.Palette.Dim)
		g.text(screen, city.Point{X: x, Y: y + 2*lineH}, "Next: "+oneLine(e.Next), ui.Small, th.Palette.Text)
		y += 3*lineH + grid
	}
	footer := "enter goes there  ·  r restores the desk  ·  esc to the map  ·  " + g.findChords().Journal.String() + " brings this back"
	if j.note != "" {
		footer = j.note + "  ·  " + footer
	}
	g.text(screen, city.Point{X: x, Y: plate.Max.Y - lineH - grid}, footer, ui.Small, th.Palette.Dim)
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// sinceStrip is what changed while you were away, above the journal:
// since when, then what finished, what failed and what needed you,
// leaving out what did not happen.
func sinceStrip(since time.Time, away []events.Event, loc *time.Location) string {
	finished, failed, needed := 0, 0, 0
	for _, e := range away {
		if e.At.Before(since) {
			continue
		}
		switch e.Kind {
		case events.Ended, events.Merged:
			finished++
		case events.Error:
			failed++
		case events.NeedsYou:
			needed++
		}
	}
	parts := []string{"since " + since.In(loc).Format("Monday 15:04")}
	for _, c := range []struct {
		n    int
		what string
	}{{finished, "finished"}, {failed, "failed"}, {needed, "needed you"}} {
		if c.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c.n, c.what))
		}
	}
	return strings.Join(parts, "  ·  ")
}

// replyReader reads the last paragraph of a session's last reply from its
// transcript, when the journal opens; nothing is kept.
func (g *Game) replyReader() func(string) string {
	if g.lastReply != nil {
		return g.lastReply
	}
	home := g.home
	return func(id string) string {
		path, ok := transcriptFor(home, id)
		if !ok {
			return ""
		}
		conv, err := claude.ReadConversation(path, 0, 4)
		if err != nil {
			return ""
		}
		for i := len(conv.Messages) - 1; i >= 0; i-- {
			if conv.Messages[i].From == claude.FromClaude {
				return conv.Messages[i].Text
			}
		}
		return ""
	}
}
