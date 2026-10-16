package render

import (
	"fmt"
	"image"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/exp/textinput"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/find"
	"botropolis.city/botropolis/pkg/format"
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
	return g.just(ebiten.KeyK) && g.ctrlHeld()
}

// ctrlHeld reports Ctrl, or ⌘ on macOS, held by hand or by the script.
func (g *Game) ctrlHeld() bool {
	held := ebiten.KeyControl
	if runtime.GOOS == "darwin" {
		held = ebiten.KeyMeta
	}
	return g.scriptedCtrl || ebiten.IsKeyPressed(held)
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
	f := &finder{now: timeNow, pause: findPause}
	if g.findAtOnce {
		f.pause = 0
	}
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

// finder is the panel that finds a session by what was said in it. It
// also holds the map's filter, which is what the / box held before it.
type finder struct {
	field field
	// ime carries typing, input methods included; without a window to
	// type into, as under test, the scripted rune does.
	ime *textinput.Composer
	now func() time.Time
	// pause is how long typing rests before a search: findPause, or none
	// for a scripted frame.
	pause time.Duration
	// typedAt is when the query last changed, and asked the last query
	// sent to the search, which runs once typing pauses.
	typedAt time.Time
	asked   string

	// armed is the session whose deleted conversation Enter has offered
	// to start again, so a second Enter does it.
	armed string

	// mu guards what the search hands back from off the frame; found
	// counts deliveries and shown the one the map was last set to.
	mu       sync.Mutex
	query    string
	results  []find.Hit
	selected int
	found    int
	shown    int
}

// findPause is how long typing rests before the finder searches, so a
// word typed at speed is one search rather than one a letter.
const findPause = 150 * time.Millisecond

// deliver takes a search's results, unless the query has moved on since
// it was asked, in which case they answer a question nobody is asking.
func (f *finder) deliver(query string, hits []find.Hit) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if query != f.query {
		return
	}
	f.results, f.selected = hits, 0
	f.found++
}

// show dims the map to what the search last found, on the frame, once
// for each delivery: the finder answers from what was said, which the
// map's own filter cannot see.
func (f *finder) show(g *Game) {
	f.mu.Lock()
	if f.found == f.shown {
		f.mu.Unlock()
		return
	}
	f.shown = f.found
	query := f.query
	ids := make([]string, 0, len(f.results))
	for _, h := range f.results {
		ids = append(ids, h.SessionID)
	}
	f.mu.Unlock()
	g.query = query
	g.scene.SetFound(query, ids)
}

// chosen is the selected result, if there is one.
func (f *finder) chosen() (find.Hit, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.selected >= len(f.results) {
		return find.Hit{}, false
	}
	return f.results[f.selected], true
}

// open acts on a result and reports whether the finder stays open: a
// live session is attached, a stopped one resumed in a terminal in its
// own directory, and one whose conversation is gone is offered a new
// session where it was, which a second Enter starts.
func (f *finder) open(g *Game, h find.Hit) bool {
	switch {
	case h.Gone && f.armed != h.SessionID:
		f.armed = h.SessionID
		g.SetStatus("its conversation was deleted; enter again starts a new session in " + h.Dir)
		return true
	case h.Gone:
		g.act(city.Action{Kind: city.ActionStartIn, Dir: h.Dir})
	case find.Live(h.State):
		g.act(city.Action{Kind: city.ActionAttach, SessionID: h.SessionID})
	default:
		g.act(city.Action{Kind: city.ActionResumeIn, SessionID: h.SessionID, Dir: h.Dir})
	}
	f.end()
	return false
}

// resumeLine is the command that takes a terminal back into a session.
func resumeLine(h find.Hit) string {
	return "cd " + shellQuote(h.Dir) + " && claude --resume " + shellQuote(h.SessionID)
}

// shellQuote leaves a word the shell reads as itself alone and quotes
// anything else.
func shellQuote(s string) string {
	if s != "" && strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_./:@%+=,-") == "" {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// follow notes a change of query, clearing the results when it is
// emptied, and asks for results once typing has paused.
func (f *finder) follow(g *Game) {
	query := f.field.query()
	f.mu.Lock()
	emptied := false
	if query != f.query {
		f.query, f.typedAt = query, f.now()
		if query == "" {
			f.results, f.selected, f.asked, emptied = nil, 0, "", true
		}
	}
	f.mu.Unlock()
	if emptied {
		g.filterOn("")
	}
	if query == "" || query == f.asked || f.now().Sub(f.typedAt) < f.pause || g.search == nil {
		return
	}
	f.asked = query
	search, dispatch := g.search, g.dispatch
	if dispatch == nil {
		dispatch = func(run func()) { go run() }
	}
	dispatch(func() {
		hits, err := search(find.Query{Words: query, Limit: findLimit})
		if err != nil {
			g.SetStatus(err.Error())
			return
		}
		f.deliver(query, hits)
	})
}

// findLimit is how many sessions the finder asks for.
const findLimit = 50

// move steps the selection, stopping at either end.
func (f *finder) move(by int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.selected = max(0, min(f.selected+by, len(f.results)-1))
}

func (f *finder) keys(g *Game) bool {
	just := g.just
	if f.ime != nil {
		if handled, err := f.ime.Update(); err != nil {
			g.SetStatus(err.Error())
		} else if handled {
			f.follow(g)
			f.provisional(g)
			f.show(g)
			return true
		}
	}
	f.show(g)
	h, chosen := f.chosen()
	switch {
	case just(ebiten.KeyEscape):
		f.end()
		g.filterOn("")
		return false
	case chosen && g.ctrlHeld() && just(ebiten.KeyY):
		line := resumeLine(h)
		g.SetStatus("copied: " + line)
		g.act(city.Action{Kind: city.ActionCopy, Text: line})
		return true
	case chosen && g.ctrlHeld() && just(ebiten.KeyM):
		f.end()
		if !g.scene.Select(h.SessionID) {
			g.SetStatus("that session is not on the map")
		}
		return false
	case just(ebiten.KeyEnter), just(ebiten.KeyKPEnter):
		if chosen {
			return f.open(g, h)
		}
		f.end()
		return false
	case just(ebiten.KeyBackspace):
		f.field.backspace()
	case just(ebiten.KeyArrowDown):
		f.move(1)
	case just(ebiten.KeyArrowUp):
		f.move(-1)
	}
	if g.scriptedRune != 0 {
		f.field.insert(string(g.scriptedRune))
	}
	f.follow(g)
	f.provisional(g)
	f.show(g)
	return true
}

// provisional narrows the map by matching the words against what it
// shows — titles, projects, branches, states and models — until the
// finder's first answer arrives, so typing has an effect at once. After
// that the map holds the last answer until the next, rather than going
// dark between searches.
func (f *finder) provisional(g *Game) {
	f.mu.Lock()
	answered := f.found > 0
	f.mu.Unlock()
	if !answered {
		g.filterOn(f.field.query())
	}
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
	f.mu.Lock()
	hits, selected := f.results, f.selected
	f.mu.Unlock()
	status := "type to find a session by what was said in it"
	if f.field.query() != "" {
		status = fmt.Sprintf("%d sessions found, %d on the map   ↑ ↓ choose, enter opens, ctrl+y copies how to resume, ctrl+m shows it, esc clears",
			len(hits), g.scene.Matching())
	}
	in := ui.FinderInput{Query: shown, Caret: caret, Status: status, Words: strings.Fields(f.field.query()), Selected: selected}
	now := g.now()
	for _, h := range hits {
		in.Results = append(in.Results, finderResult(h, now))
	}
	p := ui.LayoutFinder(th, width, height, in, g.faces.Measure)
	g.mu.Lock()
	g.finderRect, g.finderCaret = p.Rect, p.Caret
	g.mu.Unlock()
	g.roundRect(screen, p.Rect, p.Radius, p.Fill)
	g.text(screen, p.Query.At, p.Query.Text, p.Query.Size, th.Palette.Text)
	g.roundRect(screen, p.Caret, 0, th.Palette.Accent)
	g.text(screen, p.Status.At, p.Status.Text, p.Status.Size, th.Palette.Dim)
	for _, row := range p.Rows {
		if row.Selected {
			g.roundRect(screen, row.Rect, 0, th.Palette.Hairline)
		}
		for _, m := range row.Marks {
			_, h := g.faces.Measure("", m.Line.Size)
			highlight := th.Palette.Accent
			highlight.A = 0x50
			g.roundRect(screen, city.Rect{Min: city.Point{X: m.From, Y: m.Line.At.Y}, Max: city.Point{X: m.To, Y: m.Line.At.Y + h}}, th.Radius()/3, highlight)
		}
		g.text(screen, row.Head.At, row.Head.Text, row.Head.Size, th.Palette.Text)
		g.text(screen, row.Where.At, row.Where.Text, row.Where.Size, th.Palette.Dim)
		if row.Passage.Text != "" {
			g.text(screen, row.Passage.At, row.Passage.Text, row.Passage.Size, th.Palette.Dim)
		}
	}
	for _, line := range p.Preview {
		g.text(screen, line.At, line.Text, line.Size, th.Palette.Dim)
	}
}

// finderResult is a hit as the panel shows it.
func finderResult(h find.Hit, now time.Time) ui.FinderResult {
	return ui.FinderResult{
		InPrompt: h.Kind == claude.SaidPrompt, Passage: h.Passage, FirstPrompt: h.FirstPrompt,
		Dir: h.Dir, Branch: h.Branch, Age: format.Age(now.Sub(h.At)), Gone: h.Gone,
		Recap: h.Recap, LastPrompt: h.LastPrompt,
	}
}

// filterOn sets the map's filter, which dims what does not match.
func (g *Game) filterOn(query string) {
	g.query = query
	g.scene.SetFilter(query)
}
