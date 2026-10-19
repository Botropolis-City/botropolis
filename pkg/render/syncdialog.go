package render

import (
	"context"
	"errors"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/ui"
)

// ErrRestart ends the city so that it can be run again from its path on
// disk, which after an upgrade is the new build (item 152).
var ErrRestart = errors.New("restart the city")

const (
	exitLabel          = "Exit"
	restartDaemonLabel = "Restart Daemon"
	restartCityLabel   = "Restart City"
)

// syncDialog stands in for the whole city while its daemon is another
// release or protocol: the two must be running the same version, so
// nothing of the city is drawn and nothing but the dialog answers. It
// offers to exit, or to restart whichever end is behind. Item 152.
type syncDialog struct {
	mismatch   *proto.Mismatch
	restarting bool
	failure    string
}

type syncWording struct {
	Title string
	Lines []string
	// Versions are the rows under the text: a label and a version.
	Versions [][2]string
	Status   string
	Buttons  []string
}

func (d *syncDialog) wording() syncWording {
	m := d.mismatch
	restart, which := restartDaemonLabel, "the daemon"
	if m.Newer() {
		restart, which = restartCityLabel, "the city"
	}
	w := syncWording{
		Title: "Botropolis is out of sync with its daemon",
		Lines: []string{"The Botropolis city and its daemon must be running the same version. Please restart " + which + " to continue."},
		Versions: [][2]string{
			{"Botropolis daemon version", m.DaemonVersion()},
			{"Botropolis city version", m.CityVersion()},
		},
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

// syncTone is how a line of the dialog is coloured.
type syncTone int

const (
	toneBody syncTone = iota
	toneLabel
	toneStatus
)

type syncText struct {
	ui.Text
	tone syncTone
}

type syncLayout struct {
	panel   city.Rect
	title   city.Point
	texts   []syncText
	buttons []ui.Button
}

// layoutSync lays the dialog out: the title, the text, the two version
// rows with their versions aligned in a column, a status when there is
// one, and the buttons at the right.
func (g *Game) layoutSync(width, height float64) syncLayout {
	th := g.theme
	grid := th.Grid()
	w := g.sync.wording()
	_, title := g.faces.Measure("", ui.Title)
	_, body := g.faces.Measure("", ui.Body)
	panelWidth := min(width-grid*8, grid*90)
	inner := panelWidth - grid*4
	var text []string
	for _, l := range w.Lines {
		text = append(text, g.wrapWords(l, inner, ui.Body)...)
	}
	var status []string
	if w.Status != "" {
		status = g.wrapWords(w.Status, inner, ui.Body)
	}
	labelWidth := 0.0
	for _, row := range w.Versions {
		lw, _ := g.faces.Measure(row[0], ui.Body)
		labelWidth = max(labelWidth, lw)
	}
	statusGap := 0.0
	if len(status) > 0 {
		statusGap = grid
	}
	panelHeight := grid*4 + title + grid + float64(len(text))*body + grid +
		float64(len(w.Versions))*body + statusGap + float64(len(status))*body + grid*2 + body + grid
	panel := city.RectAt((width-panelWidth)/2, (height-panelHeight)/2, panelWidth, panelHeight)
	x, y := panel.Min.X+grid*2, panel.Min.Y+grid*2
	l := syncLayout{panel: panel, title: city.Point{X: x, Y: y}}
	add := func(s string, at float64, tone syncTone) {
		l.texts = append(l.texts, syncText{ui.Text{Text: s, At: city.Point{X: at, Y: y}, Size: ui.Body}, tone})
	}
	y += title + grid
	for _, line := range text {
		add(line, x, toneBody)
		y += body
	}
	y += grid
	for _, row := range w.Versions {
		add(row[0], x, toneLabel)
		add(row[1], x+labelWidth+grid*3, toneBody)
		y += body
	}
	y += statusGap
	for _, line := range status {
		add(line, x, toneStatus)
		y += body
	}
	y += grid * 2
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
	for _, t := range l.texts {
		c := th.Palette.Text
		switch {
		case t.tone == toneLabel:
			c = th.Palette.Dim
		case t.tone == toneStatus && g.sync.restarting:
			c = th.Palette.Dim
		case t.tone == toneStatus:
			c = th.Palette.NeedsYou
		}
		g.text(screen, t.At, t.Text.Text, t.Size, c)
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
