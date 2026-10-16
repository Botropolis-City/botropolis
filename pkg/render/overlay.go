package render

import (
	"fmt"
	"image"
	"runtime"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/exp/textinput"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// overlay is a panel drawn over the city. The one on top of the stack
// takes every key and every click while it is there, so nothing it is
// drawn over can be reached through it. Item 82.
type overlay interface {
	// keys answers the keyboard for a tick and reports whether the
	// overlay stays open.
	keys(g *Game) bool
	// click answers a press of the pointer and reports whether the
	// overlay stays open.
	click(g *Game, at city.Point) bool
	draw(g *Game, screen *ebiten.Image, width, height float64)
}

type overlays []overlay

func (s overlays) top() overlay {
	if len(s) == 0 {
		return nil
	}
	return s[len(s)-1]
}

func (s *overlays) push(o overlay) { *s = append(*s, o) }

func (s *overlays) pop() { *s = (*s)[:len(*s)-1] }

// overlayKeys gives the keyboard to the top of the stack, and reports
// whether there was anything there to take it.
func (g *Game) overlayKeys() bool {
	top := g.overlays.top()
	if top == nil {
		return false
	}
	if !top.keys(g) {
		g.overlays.pop()
	}
	return true
}

// clickOverlay gives a press of the pointer to the top of the stack;
// while anything is on it, the map behind takes no clicks.
func (g *Game) clickOverlay(cursor city.Point) bool {
	top := g.overlays.top()
	if top == nil {
		return false
	}
	if !top.click(g, cursor) {
		g.overlays.pop()
	}
	return true
}

func (g *Game) drawOverlays(screen *ebiten.Image, width, height float64) {
	for _, o := range g.overlays {
		o.draw(g, screen, width, height)
	}
}

// paletteKey reports Ctrl+K, or ⌘K on macOS, the way command palettes
// are opened elsewhere. Nothing on the map uses Ctrl.
func (g *Game) paletteKey() bool {
	held := ebiten.KeyControl
	if runtime.GOOS == "darwin" {
		held = ebiten.KeyMeta
	}
	return g.just(ebiten.KeyK) && (g.scriptedCtrl || ebiten.IsKeyPressed(held))
}

func (g *Game) finding() bool {
	_, ok := g.overlays.top().(*finder)
	return ok
}

// openFinder puts the finder on the stack, holding the filter as it
// stands so a second look carries on from the first. It stands the
// quit prompt down: a prompt the panel covers is one nobody answers.
func (g *Game) openFinder() {
	if g.quit.armed(timeNow()) {
		g.quit.cancel()
		g.clearPrompt()
	}
	if g.finding() {
		return
	}
	f := &finder{}
	f.field.insert(g.query)
	if g.ime {
		f.ime = &textinput.Composer{
			OnNewSession: func() *textinput.SessionOptions {
				g.mu.Lock()
				caret := g.finderCaret
				g.mu.Unlock()
				before := string(f.field.text[:f.field.caret])
				return &textinput.SessionOptions{
					CaretBounds:     image.Rect(int(caret.Min.X), int(caret.Min.Y), int(caret.Max.X)+1, int(caret.Max.Y)),
					TextBeforeCaret: before,
					TextAfterCaret:  string(f.field.text[f.field.caret:]),
				}
			},
			OnComposition: func(c *textinput.Composition) { f.field.compose(c.Text()) },
			OnCommit:      func(c *textinput.Commit) { f.field.insert(c.Text()) },
		}
	}
	g.overlays.push(f)
}

// finder is the panel that finds a session. In this slice it holds the
// map's filter, which is what the / box held before it.
type finder struct {
	field field
	// ime carries typing, input methods included; without a window to
	// type into, as under test, the scripted rune does.
	ime *textinput.Composer
}

func (f *finder) keys(g *Game) bool {
	just := g.just
	if f.ime != nil {
		if handled, err := f.ime.Update(); err != nil {
			g.SetStatus(err.Error())
		} else if handled {
			g.filterOn(f.field.query())
			return true
		}
	}
	switch {
	case just(ebiten.KeyEscape):
		f.end()
		g.filterOn("")
		return false
	case just(ebiten.KeyEnter), just(ebiten.KeyKPEnter):
		f.end()
		return false
	case just(ebiten.KeyBackspace):
		f.field.backspace()
	}
	if g.scriptedRune != 0 {
		f.field.insert(string(g.scriptedRune))
	}
	g.filterOn(f.field.query())
	return true
}

// click closes the finder when the press lands outside it, keeping the
// filter, so the map it was narrowing is the map that is left.
func (f *finder) click(g *Game, at city.Point) bool {
	g.mu.Lock()
	panel := g.finderRect
	g.mu.Unlock()
	if panel.Contains(at) {
		return true
	}
	f.end()
	return false
}

func (f *finder) end() {
	if f.ime != nil {
		f.ime.Cancel()
	}
	f.field.compose("")
}

func (f *finder) draw(g *Game, screen *ebiten.Image, width, height float64) {
	th := g.theme
	scrim := th.Palette.Panel
	scrim.A = 0x60
	g.roundRect(screen, city.RectAt(0, 0, width, height), 0, scrim)
	shown, caret := f.field.shown()
	status := "type to filter by title, project, branch, state or model"
	if f.field.query() != "" {
		status = fmt.Sprintf("%d of %d match   enter keeps the filter, esc clears it", g.scene.Matching(), len(g.scene.City().Buildings()))
	}
	p := ui.LayoutFinder(th, width, height, ui.FinderInput{Query: shown, Caret: caret, Status: status}, g.faces.Measure)
	g.mu.Lock()
	g.finderRect, g.finderCaret = p.Rect, p.Caret
	g.mu.Unlock()
	g.roundRect(screen, p.Rect, p.Radius, p.Fill)
	g.text(screen, p.Query.At, p.Query.Text, p.Query.Size, th.Palette.Text)
	g.roundRect(screen, p.Caret, 0, th.Palette.Accent)
	g.text(screen, p.Status.At, p.Status.Text, p.Status.Size, th.Palette.Dim)
}

// filterOn sets the map's filter, which dims what does not match.
func (g *Game) filterOn(query string) {
	g.query = query
	g.scene.SetFilter(query)
}
