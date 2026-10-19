package render

import (
	"context"
	"errors"
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/ui"
)

// ErrRestart ends the map so that it can be run again from its path on
// disk, which after an upgrade is the new build (item 152).
var ErrRestart = errors.New("restart botropolis")

const (
	exitLabel           = "Exit"
	restartDaemonLabel  = "Restart Daemon"
	restartBotropolisLb = "Restart Botropolis"
)

// syncDialog stands in for the whole map while its daemon is another
// release or protocol: running the two out of sync could go wrong in
// ways nobody tested, so nothing of the city is drawn and nothing but
// the dialog answers. It offers to exit, or to restart whichever end is
// behind. Item 152.
type syncDialog struct {
	mismatch   *proto.Mismatch
	restarting bool
	failure    string
}

type syncWording struct {
	Title   string
	Lines   []string
	Status  string
	Buttons []string
}

func (d *syncDialog) wording() syncWording {
	m := d.mismatch
	versions := fmt.Sprintf("The daemon is %s and this is %s.", m.Daemon.Release, m.Ours.Release)
	switch m.Daemon.Release {
	case "":
		versions = fmt.Sprintf("The daemon is older than %s, which this is.", m.Ours.Release)
	case m.Ours.Release:
		versions = fmt.Sprintf("The daemon speaks protocol %d and this speaks %d.", m.Daemon.Protocol, m.Ours.Protocol)
	}
	why := "Running them out of sync could go wrong in ways nobody tested, so the city waits until they match."
	w := syncWording{Title: "Botropolis is out of sync with its daemon", Lines: []string{versions + " " + why}}
	restart := restartDaemonLabel
	if m.Newer() {
		restart = restartBotropolisLb
		w.Lines = append(w.Lines, "The daemon is the newer one, so this is what needs restarting.")
	}
	switch {
	case d.restarting:
		w.Status = "Restarting the daemon…"
		return w
	case d.failure != "":
		w.Status = "The restart failed: " + d.failure
	}
	w.Buttons = []string{exitLabel, restart}
	return w
}

// OutOfSync is the feed saying its daemon is another release or
// protocol; the next snapshot it offers says one of this build's
// answered.
func (g *Game) OutOfSync(m *proto.Mismatch) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.pendingSync, g.syncChanged = m, true
}

// restartDone is a restart's outcome, from whatever ran it.
func (g *Game) restartDone(err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.restartErr, g.restartFinished = err, true
}

// takeSync brings what the feed and a restart said into the dialog, on
// the game's own thread.
func (g *Game) takeSync() {
	g.mu.Lock()
	pending, changed := g.pendingSync, g.syncChanged
	finished, failed := g.restartFinished, g.restartErr
	g.syncChanged, g.restartFinished, g.restartErr = false, false, nil
	g.mu.Unlock()
	if changed {
		switch {
		case pending == nil:
			g.sync = nil
		case g.sync == nil:
			g.sync = &syncDialog{mismatch: pending}
			if (g.screenshot != "" || g.record != "") && g.shotFrames == 0 {
				g.shotFrames = 1
			}
		default:
			g.sync.mismatch = pending
		}
	}
	if finished && failed != nil && g.sync != nil {
		g.sync.restarting, g.sync.failure = false, failed.Error()
	}
}

// cityShown is whether the city is drawn at all.
func (g *Game) cityShown() bool { return g.sync == nil }

// syncInput answers the dialog: Escape or q exits, Enter or r restarts,
// and so do its buttons.
func (g *Game) syncInput(cursor city.Point, clicked bool) error {
	if g.sync.restarting {
		return nil
	}
	switch {
	case g.just(ebiten.KeyEscape), g.just(ebiten.KeyQ):
		return ebiten.Termination
	case g.just(ebiten.KeyEnter), g.just(ebiten.KeyR):
		return g.syncRestart()
	}
	if !clicked {
		return nil
	}
	size := g.scene.Size()
	if b, ok := ui.HitButton(g.syncButtons(size.X, size.Y), cursor); ok {
		if b.Label.Text == exitLabel {
			return ebiten.Termination
		}
		return g.syncRestart()
	}
	return nil
}

func (g *Game) syncRestart() error {
	d := g.sync
	if d.mismatch.Newer() {
		return ErrRestart
	}
	d.restarting, d.failure = true, ""
	restart := g.restartDaemon
	if restart == nil {
		restart = func(context.Context) error { return errors.New("nothing here can restart it") }
	}
	dispatch := g.dispatch
	if dispatch == nil {
		dispatch = func(run func()) { go run() }
	}
	dispatch(func() { g.restartDone(restart(context.Background())) })
	g.takeSync()
	return nil
}

type syncLayout struct {
	panel   city.Rect
	title   city.Point
	lines   []ui.Text
	status  ui.Text
	buttons []ui.Button
}

func (g *Game) layoutSync(width, height float64) syncLayout {
	th := g.theme
	grid := th.Grid()
	w := g.sync.wording()
	_, title := g.faces.Measure("", ui.Title)
	_, body := g.faces.Measure("", ui.Body)
	panelWidth := min(width-grid*8, grid*90)
	inner := panelWidth - grid*4
	var wrapped [][]string
	rows := 0
	for _, l := range w.Lines {
		lines := g.wrapWords(l, inner, ui.Body)
		wrapped = append(wrapped, lines)
		rows += len(lines)
	}
	var status []string
	if w.Status != "" {
		status = g.wrapWords(w.Status, inner, ui.Body)
	}
	buttonRow := body + grid
	panelHeight := grid*4 + title + grid + float64(rows)*body + float64(len(wrapped))*grid/2 +
		float64(len(status))*body + grid + buttonRow
	panel := city.RectAt((width-panelWidth)/2, (height-panelHeight)/2, panelWidth, panelHeight)
	x, y := panel.Min.X+grid*2, panel.Min.Y+grid*2
	l := syncLayout{panel: panel, title: city.Point{X: x, Y: y}}
	y += title + grid
	for _, para := range wrapped {
		for _, line := range para {
			l.lines = append(l.lines, ui.Text{Text: line, At: city.Point{X: x, Y: y}, Size: ui.Body})
			y += body
		}
		y += grid / 2
	}
	for _, line := range status {
		l.lines = append(l.lines, ui.Text{Text: line, At: city.Point{X: x, Y: y}, Size: ui.Body})
		y += body
	}
	if len(status) > 0 {
		l.status = l.lines[len(l.lines)-len(status)]
	}
	y += grid
	row := ui.LayoutButtons(th, w.Buttons, city.Point{X: x, Y: y}, g.faces.Measure)
	if n := len(row); n > 0 {
		shift := panel.Max.X - grid*2 - row[n-1].Rect.Max.X
		for i := range row {
			row[i].Rect = city.RectAt(row[i].Rect.Min.X+shift, row[i].Rect.Min.Y, row[i].Rect.Width(), row[i].Rect.Height())
			row[i].Label.At.X += shift
		}
	}
	l.buttons = row
	return l
}

// syncButtons is the dialog's buttons as laid out on a screen of the
// given size.
func (g *Game) syncButtons(width, height float64) []ui.Button {
	return g.layoutSync(width, height).buttons
}

func (g *Game) drawSync(screen *ebiten.Image, width, height float64) {
	th := g.theme
	screen.Fill(colorVoid)
	l := g.layoutSync(width, height)
	g.roundRect(screen, l.panel, th.Radius(), th.Palette.Panel)
	g.text(screen, l.title, g.sync.wording().Title, ui.Title, th.Palette.Text)
	statusFrom := len(l.lines)
	if l.status.Text != "" {
		for i, t := range l.lines {
			if t == l.status {
				statusFrom = i
			}
		}
	}
	for i, t := range l.lines {
		c := th.Palette.Text
		if i >= statusFrom {
			c = th.Palette.NeedsYou
			if g.sync.restarting {
				c = th.Palette.Dim
			}
		}
		g.text(screen, t.At, t.Text, t.Size, c)
	}
	for i, b := range l.buttons {
		fill, ink := th.Palette.Hairline, th.Palette.Text
		if i == len(l.buttons)-1 {
			fill, ink = th.Palette.Accent, th.Palette.Panel
			ink.A = 0xff
		}
		g.roundRect(screen, b.Rect, th.Radius(), fill)
		g.text(screen, b.Label.At, b.Label.Text, b.Label.Size, ink)
	}
}
