package render

import (
	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// quitPanel asks before the city quits: a small panel in the middle of
// the window, over the dimmed city, with Quit, Close up… and Cancel. It
// stays until it is answered, however slow the loop (items 270, 292),
// and anything that asks to quit opens it with openQuit.
type quitPanel struct{}

// quitChoice is a button on the panel and the key it stands for: a click
// presses the key, so the click and the keyboard run one path.
type quitChoice struct {
	label, keys string
	key         ebiten.Key
}

var quitChoices = []quitChoice{
	{"Quit", "y · enter", ebiten.KeyY},
	{"Close up…", "c", ebiten.KeyC},
	{"Cancel", "n · esc", ebiten.KeyN},
}

const quitTitle = "Quit Botropolis?"

// quitLayout is the panel as laid out: its plate, its buttons, and under
// each button the keys that press it.
type quitLayout struct {
	Plate   city.Rect
	Buttons []ui.Button
	Keys    []ui.Text
}

// openQuit puts the quit panel on top of the stack, once, whatever is
// open: a close request can come with any panel up (item 293).
func (g *Game) openQuit() {
	g.standQuitDown()
	g.overlays.push(&quitPanel{})
}

func (g *Game) quitPanel() *quitPanel {
	p, _ := g.overlays.top().(*quitPanel)
	return p
}

// standQuitDown takes the quit panel off the stack, wherever it is.
func (g *Game) standQuitDown() {
	for i, o := range g.overlays {
		if _, ok := o.(*quitPanel); ok {
			g.overlays = append(g.overlays[:i], g.overlays[i+1:]...)
			return
		}
	}
}

func (p *quitPanel) layout(g *Game, width, height float64) quitLayout {
	grid, lineH, measure := g.metrics()
	widths := make([]float64, len(quitChoices))
	row := 0.0
	for i, c := range quitChoices {
		label, _ := measure(c.label, ui.Body)
		keys, _ := measure(c.keys, ui.Small)
		widths[i] = max(label, keys) + 4*grid
		row += widths[i]
	}
	row += float64(len(quitChoices)-1) * 2 * grid
	titleW, _ := measure(quitTitle, ui.Body)
	w := min(width-8*grid, max(row, titleW)+6*grid)
	buttonH := lineH + 2*grid
	h := 3*grid + lineH + 2*grid + buttonH + grid + lineH + 2*grid
	plate := city.RectAt((width-w)/2, (height-h)/2, w, h)
	x := plate.Min.X + (w-row)/2
	y := plate.Min.Y + 3*grid + lineH + 2*grid
	l := quitLayout{Plate: plate}
	for i, c := range quitChoices {
		label, _ := measure(c.label, ui.Body)
		keys, _ := measure(c.keys, ui.Small)
		l.Buttons = append(l.Buttons, ui.Button{
			Rect:  city.RectAt(x, y, widths[i], buttonH),
			Label: ui.Text{Text: c.label, At: city.Point{X: x + (widths[i]-label)/2, Y: y + grid}, Size: ui.Body},
		})
		l.Keys = append(l.Keys, ui.Text{Text: c.keys, At: city.Point{X: x + (widths[i]-keys)/2, Y: y + buttonH + grid}, Size: ui.Small})
		x += widths[i] + 2*grid
	}
	return l
}

func (p *quitPanel) keys(g *Game) bool {
	switch {
	case g.just(ebiten.KeyY), g.just(ebiten.KeyEnter), g.just(ebiten.KeyKPEnter):
		g.leaving = true
	case g.just(ebiten.KeyC):
		g.standQuitDown()
		g.openCloseUp()
	case g.just(ebiten.KeyN), g.just(ebiten.KeyEscape), g.just(ebiten.KeyQ):
		return false
	}
	return true
}

// click presses the key of the button under the pointer; anywhere else
// it does nothing, and the panel stays.
func (p *quitPanel) click(g *Game, at city.Point) bool {
	size := g.scene.Size()
	for i, b := range p.layout(g, size.X, size.Y).Buttons {
		if b.Rect.Contains(at) {
			g.clicked = quitChoices[i].key
		}
	}
	return true
}

func (p *quitPanel) draw(g *Game, screen *ebiten.Image, width, height float64) {
	th := g.theme
	scrim := th.Palette.Panel
	scrim.A = 0xa0
	g.roundRect(screen, city.RectAt(0, 0, width, height), 0, scrim)
	grid, _, measure := g.metrics()
	l := p.layout(g, width, height)
	plate := th.Palette.Panel
	plate.A = 0xff
	g.roundRect(screen, l.Plate, th.Radius(), plate)
	tw, _ := measure(quitTitle, ui.Body)
	g.text(screen, city.Point{X: l.Plate.Center().X - tw/2, Y: l.Plate.Min.Y + 3*grid}, quitTitle, ui.Body, th.Palette.Text)
	pointer := g.pointer()
	for i, b := range l.Buttons {
		tone := th.Palette.Text
		if i == 0 {
			tone = th.Palette.Accent
		}
		g.button(screen, b.Rect, b.Label, tone, b.Rect.Contains(pointer))
		g.text(screen, l.Keys[i].At, l.Keys[i].Text, ui.Small, th.Palette.Dim)
	}
}
