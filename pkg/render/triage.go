package render

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// triage walks what is your move (item 88), a tenant of the overlay
// stack whose keys are its own: enter acts, p rests, d marks done, s
// snoozes, n moves on, esc leaves. While walking, none of them means
// what it means on the map.
type triage struct {
	// snoozing is the snooze picker, open over the walk.
	snoozing *snoozePicker
}

type snoozePicker struct {
	choices []city.SnoozeChoice
	cursor  int
	typed   field
	refused string
}

func (g *Game) triage() *triage {
	t, _ := g.overlays.top().(*triage)
	return t
}

// startTriage begins the walk, or says there is nothing to walk.
func (g *Game) startTriage() {
	if g.triage() != nil {
		return
	}
	if !g.scene.StartTriage() {
		g.SetStatus("nothing waits on you")
		return
	}
	g.overlays.push(&triage{})
}

func (t *triage) keys(g *Game) bool {
	if t.snoozing != nil {
		t.snoozeKeys(g)
		return true
	}
	switch {
	case g.just(ebiten.KeyEscape):
		return false
	case g.just(ebiten.KeyEnter), g.just(ebiten.KeyKPEnter):
		action, note := g.scene.TriageAct()
		g.noteAndAct(action, note)
	case g.just(ebiten.KeyP):
		action, note := g.scene.TriageRest()
		g.noteAndAct(action, note)
		return g.scene.TriageNext()
	case g.just(ebiten.KeyD):
		g.act(g.scene.TriageDone())
		return g.stillWalking()
	case g.just(ebiten.KeyS):
		t.snoozing = &snoozePicker{choices: city.SnoozeChoices(g.scene.Now())}
	case g.just(ebiten.KeyN):
		return g.scene.TriageNext()
	}
	return true
}

func (g *Game) noteAndAct(action city.Action, note string) {
	if note != "" {
		g.SetStatus(note)
	}
	g.act(action)
}

// stillWalking reports whether the walk has anything left, and says so
// when it has not.
func (g *Game) stillWalking() bool {
	if _, ok := g.scene.TriageStep(); ok {
		return true
	}
	g.SetStatus("nothing else waits on you")
	return false
}

func (t *triage) snoozeKeys(g *Game) {
	p := t.snoozing
	typing := p.cursor == len(p.choices)
	switch {
	case g.just(ebiten.KeyEscape):
		t.snoozing = nil
		return
	case g.just(ebiten.KeyArrowDown):
		p.cursor = min(p.cursor+1, len(p.choices))
	case g.just(ebiten.KeyArrowUp):
		p.cursor = max(p.cursor-1, 0)
	case g.just(ebiten.KeyBackspace) && typing:
		p.typed.backspace()
	case g.just(ebiten.KeyEnter), g.just(ebiten.KeyKPEnter):
		var until time.Time
		if typing {
			at, err := city.ParseSnooze(p.typed.query(), g.scene.Now())
			if err != nil {
				p.refused = err.Error()
				return
			}
			until = at
		} else {
			until = p.choices[p.cursor].At
		}
		if err := g.scene.TriageSnooze(until); err != nil {
			p.refused = err.Error()
			return
		}
		t.snoozing = nil
		g.SetStatus("snoozed until " + until.Format("Mon 15:04"))
		if !g.stillWalking() {
			g.overlays.pop()
		}
		return
	}
	if typing {
		if g.scriptedRune != 0 {
			p.typed.insert(string(g.scriptedRune))
		}
		for _, r := range ebiten.AppendInputChars(nil) {
			p.typed.insert(string(r))
		}
	}
}

// click lets a press through to the map's card, so its buttons work
// while walking; a click off the card leaves the walk.
func (t *triage) click(g *Game, at city.Point) bool {
	if t.snoozing != nil {
		return true
	}
	return g.clickCard(at)
}

func (t *triage) draw(g *Game, screen *ebiten.Image, width, height float64) {
	if t.snoozing == nil {
		return
	}
	th := g.theme
	p := t.snoozing
	labels := make([]string, len(p.choices))
	for i, c := range p.choices {
		labels[i] = c.Label
	}
	shown, _ := p.typed.shown()
	laid := ui.LayoutSnoozePicker(th, width, height, labels, p.cursor, shown, p.refused, g.faces.Measure)
	vector.FillRect(screen, 0, 0, float32(width), float32(height), colorScrim, false)
	g.roundPanel(screen, laid.Rect)
	g.run(screen, laid.Title, th.Palette.Text)
	for i, r := range laid.Rows {
		if i == laid.Cursor {
			g.roundRect(screen, laid.Bands[i], th.Radius()/2, th.Palette.Hairline)
		}
		g.run(screen, r, th.Palette.Text)
	}
	g.run(screen, laid.Error, th.Palette.Error)
}
