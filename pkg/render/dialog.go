package render

import (
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// dialog is a prompt: anything with a choice or a loss asks in one, a
// plate in the middle of the window over the dimmed city, with its
// buttons and under each the keys that press it. It stays until it is
// answered, however slow the loop (items 270, 292, 352). The quit
// panel was the first; demolishing and closing a group ask the same way.
type dialog struct {
	title   string
	lines   []string
	choices []dialogChoice
	// plate is where the dialog was last drawn, for the frame's probe.
	plate city.Rect
}

// dialogChoice is one button: its label, the keys that answer it, which
// the text under the button is spelled from so the two cannot disagree,
// and what it does, which reports whether the dialog stays open. A nil
// run closes the dialog and does nothing else.
type dialogChoice struct {
	label string
	keys  []ebiten.Key
	run   func(g *Game) bool
}

// answers is whether the choice takes a key.
func (c dialogChoice) answers(key ebiten.Key) bool {
	for _, k := range c.keys {
		if k == key {
			return true
		}
	}
	return false
}

// keyAliases are keys a choice answers that stand for another, which
// the text under the button does not spell: the keypad's enter is enter.
var keyAliases = map[ebiten.Key]ebiten.Key{ebiten.KeyKPEnter: ebiten.KeyEnter}

// keyName is a key as the text under a button spells it, and as press
// reads it back.
func keyName(key ebiten.Key) string {
	switch key {
	case ebiten.KeyEscape:
		return "esc"
	case ebiten.KeyEnter:
		return "enter"
	case ebiten.KeyDelete:
		return "delete"
	}
	return strings.ToLower(key.String())
}

// keysText spells a choice's keys under its button.
func (c dialogChoice) keysText() string {
	var names []string
	for _, k := range c.keys {
		if _, alias := keyAliases[k]; !alias {
			names = append(names, keyName(k))
		}
	}
	return strings.Join(names, " · ")
}

const quitTitle = "Quit Botropolis?"

// quitDialog asks before the city quits: Quit, Close up… and Cancel.
// q cancels it too, since q opened it, and says so on the button.
func quitDialog() *dialog {
	return &dialog{title: quitTitle, choices: []dialogChoice{
		{"Quit", []ebiten.Key{ebiten.KeyY, ebiten.KeyEnter, ebiten.KeyKPEnter}, func(g *Game) bool { g.leaving = true; return true }},
		{"Close up…", []ebiten.Key{ebiten.KeyC}, func(g *Game) bool { g.openCloseUp(); return true }},
		{"Cancel", []ebiten.Key{ebiten.KeyN, ebiten.KeyEscape, ebiten.KeyQ}, nil},
	}}
}

// demolishDialog asks before a session is demolished; d again confirms,
// so d d still demolishes, and the first d is what shows the question.
func demolishDialog(title, id string) *dialog {
	return &dialog{title: "Demolish " + title + "?", lines: []string{city.ActionDemolish.Describe()}, choices: []dialogChoice{
		{"Demolish", []ebiten.Key{ebiten.KeyD, ebiten.KeyDelete, ebiten.KeyEnter, ebiten.KeyKPEnter}, func(g *Game) bool {
			g.act(city.Action{Kind: city.ActionDemolish, SessionID: id})
			return false
		}},
		{"Cancel", []ebiten.Key{ebiten.KeyEscape, ebiten.KeyN}, nil},
	}}
}

// closeGroupDialog asks before a group is closed; Delete again confirms.
func closeGroupDialog(name, id string) *dialog {
	return &dialog{title: "Close " + name + "?", lines: []string{city.ActionCloseGroup.Describe()}, choices: []dialogChoice{
		{"Close", []ebiten.Key{ebiten.KeyDelete, ebiten.KeyEnter, ebiten.KeyKPEnter}, func(g *Game) bool {
			action, note := g.scene.CloseGroup(id)
			g.act(action)
			g.Notify(note, ui.ToneNone, "")
			return false
		}},
		{"Cancel", []ebiten.Key{ebiten.KeyEscape, ebiten.KeyN}, nil},
	}}
}

// askDemolish is d or Delete on the map: it asks about the selection, or
// says there is none.
func (g *Game) askDemolish() {
	b := g.scene.Selected()
	if b == nil {
		g.Notify("select a building first", ui.ToneNone, "")
		return
	}
	g.overlays.push(demolishDialog(b.Card(g.scene.Now()).Title, b.Session.ID))
}

// dialog is the dialog on top of the stack, if one is.
func (g *Game) dialog() *dialog {
	d, _ := g.overlays.top().(*dialog)
	return d
}

// openQuit puts the quit dialog on top of the stack, once, whatever is
// open: a close request can come with any panel up (item 293).
func (g *Game) openQuit() {
	g.standQuitDown()
	g.overlays.push(quitDialog())
}

// quitPanel is the quit dialog while it is on top.
func (g *Game) quitPanel() *dialog {
	if d := g.dialog(); d != nil && d.title == quitTitle {
		return d
	}
	return nil
}

// standQuitDown takes the quit dialog off the stack, wherever it is.
func (g *Game) standQuitDown() {
	for i, o := range g.overlays {
		if d, ok := o.(*dialog); ok && d.title == quitTitle {
			g.overlays = append(g.overlays[:i], g.overlays[i+1:]...)
			return
		}
	}
}

func (d *dialog) layout(g *Game, width, height float64) ui.Dialog {
	_, _, measure := g.metrics()
	choices := make([]ui.DialogChoice, len(d.choices))
	for i, c := range d.choices {
		choices[i] = ui.DialogChoice{Label: c.label, Keys: c.keysText()}
	}
	return ui.LayoutDialog(g.theme, d.title, d.lines, choices, width, height, measure)
}

func (d *dialog) keys(g *Game) bool {
	for _, c := range d.choices {
		for _, k := range c.keys {
			if g.just(k) {
				if c.run == nil {
					return false
				}
				return c.run(g)
			}
		}
	}
	return true
}

// click presses the first key of the button under the pointer, so the
// click and the keyboard run one path; anywhere else it does nothing,
// and the dialog stays.
func (d *dialog) click(g *Game, at city.Point) bool {
	size := g.scene.Size()
	for i, b := range d.layout(g, size.X, size.Y).Buttons {
		if b.Rect.Contains(at) && len(d.choices[i].keys) > 0 {
			g.clicked = d.choices[i].keys[0]
		}
	}
	return true
}

func (d *dialog) draw(g *Game, screen *ebiten.Image, width, height float64) {
	th := g.theme
	scrim := th.Palette.Panel
	scrim.A = 0xa0
	g.roundRect(screen, city.RectAt(0, 0, width, height), 0, scrim)
	l := d.layout(g, width, height)
	d.plate = l.Plate
	plate := th.Palette.Panel
	plate.A = 0xff
	g.roundRect(screen, l.Plate, th.Radius(), plate)
	g.run(screen, l.Title, th.Palette.Text)
	for _, line := range l.Lines {
		g.run(screen, line, th.Palette.Dim)
	}
	pointer := g.pointer()
	for i, b := range l.Buttons {
		tone := th.Palette.Text
		if i == 0 {
			tone = th.Palette.Accent
		}
		g.button(screen, b.Rect, b.Label, tone, b.Rect.Contains(pointer))
		g.run(screen, l.Keys[i], th.Palette.Dim)
	}
}
