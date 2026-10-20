package render

import (
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/stats"
	"botropolis.city/botropolis/pkg/ui"
)

// statsHooks is how the statistics prompts reach the store: whether
// statistics are on, which prompt the state directory owes, when a kept
// database was last written, and what answers a prompt. Item 110.
type statsHooks struct {
	on     bool
	owed   func(on bool) stats.Prompt
	since  func() (time.Time, bool)
	choose func(p stats.Prompt, keep bool) error
	asked  bool
}

// statsPrompt asks what to do with stored statistics when they are
// turned off, or back on over data kept while off. Nothing is deleted
// without an answer, and escape changes nothing. Item 110.
type statsPrompt struct {
	kind   stats.Prompt
	atOpen bool
	since  time.Time
}

type statsWording struct {
	Title string
	Lines []string
	Keys  string
}

func (p *statsPrompt) wording() statsWording {
	later := "esc  ask me next time"
	if p.kind == stats.OffPrompt {
		title := "Turn statistics off"
		if p.atOpen {
			title = "Statistics were turned off"
		} else {
			later = "esc  leave statistics on"
		}
		return statsWording{
			Title: title,
			Lines: []string{
				"While statistics are off, Botropolis records nothing.",
				"Messages in any transcript Claude Code deletes in that time are lost to statistics for good, so turning statistics back on later leaves a gap that may not be accurate.",
			},
			Keys: "k  keep what is stored    d  delete it    " + later,
		}
	}
	title := "Turn statistics back on"
	if p.atOpen {
		title = "Statistics are back on"
	} else {
		later = "esc  leave statistics off"
	}
	return statsWording{
		Title: title,
		Lines: []string{
			"Statistics were off from " + p.since.Format("Jan 2") + " until now, and what was stored before was kept.",
			"Messages from transcripts Claude Code deleted in that time are missing, so anything that spans the gap may not be accurate.",
		},
		Keys: "k  keep it and mark the gap    f  delete it and start fresh    " + later,
	}
}

func (g *Game) statsPrompt() *statsPrompt {
	p, _ := g.overlays.top().(*statsPrompt)
	return p
}

// maybeStatsPrompt shows, once a launch, the prompt the state directory
// owes: a setting changed where no prompt could be shown. A screenshot,
// a recording or a driven run is not asked.
func (g *Game) maybeStatsPrompt() {
	if g.stats.asked || g.share != nil {
		return
	}
	g.stats.asked = true
	if g.screenshot != "" || g.record != "" || g.driven || g.stats.owed == nil {
		return
	}
	kind := g.stats.owed(g.stats.on)
	if kind != stats.NoPrompt {
		g.overlays.push(g.newStatsPrompt(kind, true))
	}
}

// playStatsPrompt shows a prompt a recording's script cues, as the city
// would show it at open.
func (g *Game) playStatsPrompt(cue string) {
	kind := stats.OffPrompt
	if cue == "on" {
		kind = stats.OnPrompt
	}
	g.overlays.push(g.newStatsPrompt(kind, true))
}

func (g *Game) newStatsPrompt(kind stats.Prompt, atOpen bool) *statsPrompt {
	p := &statsPrompt{kind: kind, atOpen: atOpen}
	if g.stats.since != nil {
		p.since, _ = g.stats.since()
	}
	return p
}

// statsRow answers the statistics row: turning it off always asks, and
// turning it on asks when data was kept while it was off. The row shows
// its old value until the prompt is answered.
func (g *Game) statsRow(s ui.Setting) {
	g.setRow(config.KeyStats, onOff(g.stats.on))
	if s.Value == "off" {
		g.overlays.push(g.newStatsPrompt(stats.OffPrompt, false))
		return
	}
	if g.stats.owed != nil && g.stats.owed(true) == stats.OnPrompt {
		g.overlays.push(g.newStatsPrompt(stats.OnPrompt, false))
		return
	}
	if g.apply != nil {
		if err := g.apply(s); err != nil {
			g.SetStatus(err.Error())
			return
		}
	}
	g.setStatsOn(true)
	g.setRow(config.KeyStats, "on")
	g.SetStatus("statistics: on")
}

func (g *Game) setRow(key, value string) {
	for i := range g.settings.Items {
		if g.settings.Items[i].Key == key {
			g.settings.Items[i].Value = value
		}
	}
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

func (p *statsPrompt) keys(g *Game) bool {
	switch {
	case g.just(ebiten.KeyEscape):
		return false
	case g.just(ebiten.KeyK):
		p.answer(g, true)
		return false
	case p.kind == stats.OffPrompt && g.just(ebiten.KeyD), p.kind == stats.OnPrompt && g.just(ebiten.KeyF):
		p.answer(g, false)
		return false
	}
	return true
}

func (p *statsPrompt) answer(g *Game, keep bool) {
	if g.stats.choose != nil {
		if err := g.stats.choose(p.kind, keep); err != nil {
			g.SetStatus(err.Error())
			return
		}
	}
	g.setStatsOn(p.kind == stats.OnPrompt)
	g.setRow(config.KeyStats, onOff(g.stats.on))
	g.SetStatus("statistics: " + onOff(g.stats.on))
}

func (p *statsPrompt) click(g *Game, at city.Point) bool { return true }

func (p *statsPrompt) draw(g *Game, screen *ebiten.Image, width, height float64) {
	th := g.theme
	scrim := th.Palette.Panel
	scrim.A = 0xa0
	g.roundRect(screen, city.RectAt(0, 0, width, height), 0, scrim)
	w := p.wording()
	grid := th.Grid()
	_, title := g.faces.Measure("", ui.Title)
	_, body := g.faces.Measure("", ui.Body)
	_, small := g.faces.Measure("", ui.Small)
	panelWidth := min(width-grid*8, grid*90)
	inner := panelWidth - grid*4
	var lines [][]string
	rows := 0
	for _, l := range w.Lines {
		wrapped := g.wrapWords(l, inner, ui.Body)
		lines = append(lines, wrapped)
		rows += len(wrapped)
	}
	panelHeight := grid*4 + title + grid + float64(rows)*body + float64(len(lines))*grid/2 + grid + small
	panel := city.RectAt((width-panelWidth)/2, (height-panelHeight)/2, panelWidth, panelHeight)
	g.roundRect(screen, panel, th.Radius(), th.Palette.Panel)
	x, y := panel.Min.X+grid*2, panel.Min.Y+grid*2
	g.text(screen, city.Point{X: x, Y: y}, w.Title, ui.Title, th.Palette.Text)
	y += title + grid
	for _, para := range lines {
		for _, l := range para {
			g.text(screen, city.Point{X: x, Y: y}, l, ui.Body, th.Palette.Text)
			y += body
		}
		y += grid / 2
	}
	y += grid
	g.text(screen, city.Point{X: x, Y: y}, w.Keys, ui.Small, th.Palette.Accent)
}

// wrapWords breaks text into lines no wider than width.
func (g *Game) wrapWords(text string, width float64, size ui.Size) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		next := word
		if line != "" {
			next = line + " " + word
		}
		if w, _ := g.faces.Measure(next, size); w > width && line != "" {
			lines = append(lines, line)
			line = word
			continue
		}
		line = next
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}
