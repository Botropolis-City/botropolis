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
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/find"
	"botropolis.city/botropolis/pkg/format"
	"botropolis.city/botropolis/pkg/state"
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

// wheeler is an overlay that answers the scroll wheel, and reports
// whether it took it; the wheel goes to the map when it does not. Item
// 127.
type wheeler interface {
	wheel(g *Game, at city.Point, notches float64) bool
}

// overlayWheel gives the wheel to the top of the stack, when that takes
// it.
func (g *Game) overlayWheel(at city.Point, notches float64) bool {
	w, ok := g.overlays.top().(wheeler)
	return ok && w.wheel(g, at, notches)
}

// ambient is an overlay that never takes the keyboard, and takes a
// click only on itself: the lens panel (item 88).
type ambient interface {
	modal() bool
	hit(at city.Point) bool
}

func isModal(o overlay) bool {
	a, ok := o.(ambient)
	return !ok || a.modal()
}

// top is the highest overlay that takes the keyboard and the pointer.
func (s overlays) top() overlay {
	for i := len(s) - 1; i >= 0; i-- {
		if isModal(s[i]) {
			return s[i]
		}
	}
	return nil
}

func (s *overlays) push(o overlay) { *s = append(*s, o) }

// pop takes the top modal overlay off the stack.
func (s *overlays) pop() {
	for i := len(*s) - 1; i >= 0; i-- {
		if isModal((*s)[i]) {
			*s = append((*s)[:i], (*s)[i+1:]...)
			return
		}
	}
}

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
		for _, o := range g.overlays {
			if a, ok := o.(ambient); ok && a.hit(cursor) {
				o.click(g, cursor)
				return true
			}
		}
		return false
	}
	if !top.click(g, cursor) {
		g.overlays.pop()
	}
	return true
}

// drawOverlays draws the modal overlays, or with ambient set the ones
// that sit under every other panel.
func (g *Game) drawOverlays(screen *ebiten.Image, width, height float64, ambient bool) {
	for _, o := range g.overlays {
		if isModal(o) != ambient {
			g.drawPanel(o, screen, width, height)
		}
	}
}

// paletteKey reports the chord that opens the finder: Ctrl+K, or ⌘K on
// macOS, the way command palettes are opened elsewhere, unless it is
// set to another. Nothing on the map uses Ctrl.
func (g *Game) paletteKey() bool {
	return g.chordPressed(g.findChords().Open)
}

// defaultChords are the finder's chords on this system when none are
// configured.
var defaultChords = sync.OnceValue(func() config.Chords {
	chords, err := config.FindChords(config.Config{}, runtime.GOOS, KnownKey)
	if err != nil {
		panic(err)
	}
	return chords
})

func (g *Game) findChords() config.Chords {
	if g.chords != nil {
		return *g.chords
	}
	return defaultChords()
}

// chordPressed reports a chord's key pressed this frame with each of its
// modifiers held, by hand or by the script.
func (g *Game) chordPressed(c config.Chord) bool {
	key, ok := keyByName(c.Key)
	if !ok || !g.just(key) {
		return false
	}
	for _, mod := range []struct {
		want, held bool
	}{
		{c.Ctrl, g.scriptedCtrl || ebiten.IsKeyPressed(ebiten.KeyControl)},
		{c.Alt, g.scriptedAlt || ebiten.IsKeyPressed(ebiten.KeyAlt)},
		{c.Shift, g.shifted()},
		{c.Cmd, g.scriptedCmd || ebiten.IsKeyPressed(ebiten.KeyMeta)},
	} {
		if mod.want && !mod.held {
			return false
		}
	}
	return true
}

// onlyHeld reports that no Ctrl, Alt or Cmd is held beyond what a chord
// asks for, so a bare key does not answer its modified chords too.
func (g *Game) onlyHeld(c config.Chord) bool {
	held := []struct{ want, down bool }{
		{c.Ctrl, g.scriptedCtrl || ebiten.IsKeyPressed(ebiten.KeyControl)},
		{c.Alt, g.scriptedAlt || ebiten.IsKeyPressed(ebiten.KeyAlt)},
		{c.Cmd, g.scriptedCmd || ebiten.IsKeyPressed(ebiten.KeyMeta)},
	}
	for _, m := range held {
		if m.down && !m.want {
			return false
		}
	}
	return true
}

func (g *Game) finding() bool {
	_, ok := g.overlays.top().(*finder)
	return ok
}

// openFinder puts the finder on the stack, holding the filter as it
// stands so a second look carries on from the first. It stands the
// quit panel down: a question the finder covers is one nobody answers.
func (g *Game) openFinder() {
	g.standQuitDown()
	if g.finding() {
		return
	}
	// The pause is timed by the city's clock, which counts ticks when it
	// is driven or recorded, so it is the same number of frames under any
	// load. Item 91.
	f := &finder{now: g.now, pause: findPause}
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
	// opening is set when Enter came before the search for what was
	// typed had answered: the answer's top result is opened when it
	// arrives, rather than nothing or an earlier query's. Items 82, 91.
	opening bool

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
	// answered is the query the results answer.
	answered string
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
	f.results, f.selected, f.answered = hits, 0, query
	f.found++
}

// current reports whether the results answer what is typed now.
func (f *finder) current() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.answered == f.query
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
// live session is attached and a stopped one resumed, exactly as the
// card's buttons do, and one whose conversation is gone is offered a new
// session where it was, which a second Enter starts as the card's new
// session here does.
func (f *finder) open(g *Game, h find.Hit) bool {
	switch {
	case h.Gone && f.armed != h.SessionID:
		f.armed = h.SessionID
		g.SetStatus("its conversation was deleted; enter again starts a new session in " + h.Dir)
		return true
	case h.Gone:
		g.act(city.Action{Kind: city.ActionNew, Dir: h.Dir})
	case h.Packed:
		g.act(city.Action{Kind: city.ActionUnpack, SessionID: h.SessionID})
	case find.Live(h.State):
		g.act(city.Action{Kind: city.ActionAttach, SessionID: h.SessionID})
	default:
		g.act(city.Action{Kind: city.ActionResume, SessionID: h.SessionID})
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
// emptied, and asks for results once typing has paused, or at once.
func (f *finder) follow(g *Game, atOnce bool) {
	query := f.field.query()
	f.mu.Lock()
	emptied := false
	if query != f.query {
		f.query, f.typedAt = query, f.now()
		f.opening = false
		if query == "" {
			f.results, f.selected, f.asked, emptied = nil, 0, "", true
		}
	}
	f.mu.Unlock()
	if emptied {
		g.filterOn("")
	}
	if query == "" || query == f.asked || (!atOnce && f.now().Sub(f.typedAt) < f.pause) || g.search == nil {
		return
	}
	f.asked = query
	search, dispatch := g.search, g.dispatch
	if dispatch == nil {
		dispatch = func(run func()) { go run() }
	}
	scripted := g.scene.Shown().Scripted
	dispatch(func() {
		hits, err := search(find.Query{Words: query, Limit: findLimit})
		if err != nil {
			g.SetStatus(err.Error())
			return
		}
		f.deliver(query, onMapHits(hits, scripted))
	})
}

// onMapHits leaves out the sessions the map leaves out, so the finder
// never offers one it cannot show (item 273).
func onMapHits(hits []find.Hit, scripted bool) []find.Hit {
	if scripted {
		return hits
	}
	kept := hits[:0:0]
	for _, h := range hits {
		if !h.Scripted {
			kept = append(kept, h)
		}
	}
	return kept
}

// openAnswer opens the top result of the search Enter was waiting on,
// or closes the finder when it found nothing, as Enter does then.
func (f *finder) openAnswer(g *Game) bool {
	f.opening = false
	f.show(g)
	if h, ok := f.chosen(); ok {
		return f.open(g, h)
	}
	f.end()
	return false
}

// findLimit is how many sessions the finder asks for.
const findLimit = 50

// move steps the selection, stopping at either end.
func (f *finder) move(by, groups int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.selected = max(0, min(f.selected+by, len(f.results)+groups-1))
}

// groups are the groups the query names, which follow the sessions
// found (item 90).
func (f *finder) groups(g *Game) []state.Group {
	return city.GroupsMatching(g.scene.Groups(), f.field.query())
}

// chosenGroup is the selected group, when the selection is past the
// sessions found.
func (f *finder) chosenGroup(g *Game) (state.Group, bool) {
	groups := f.groups(g)
	f.mu.Lock()
	i := f.selected - len(f.results)
	f.mu.Unlock()
	if i < 0 || i >= len(groups) {
		return state.Group{}, false
	}
	return groups[i], true
}

// rows are the sessions found, then the groups named.
func (f *finder) rows(g *Game, now time.Time) []ui.FinderResult {
	f.mu.Lock()
	hits := f.results
	f.mu.Unlock()
	var out []ui.FinderResult
	for _, h := range hits {
		out = append(out, finderResult(h, now))
	}
	for _, gr := range f.groups(g) {
		out = append(out, ui.FinderResult{Group: true, InPrompt: true, Passage: gr.Name,
			Dir: "group  " + g.route(gr.ID), Age: movePhrase(gr.Move)})
	}
	return out
}

// route is a group's stations, in line order.
func (g *Game) route(id string) string {
	for _, l := range g.scene.City().Lines {
		if l.Group.ID != id {
			continue
		}
		names := make([]string, 0, len(l.Stations))
		for _, st := range l.Stations {
			names = append(names, st.District.Name)
		}
		return strings.Join(names, " → ")
	}
	return ""
}

func movePhrase(m state.Move) string {
	if m == state.MoveYours {
		return "your move"
	}
	return string(m) + " move"
}

// status is the line under the query.
func (f *finder) status(g *Game) string {
	if f.field.query() == "" {
		return "type to find a session by what was said in it"
	}
	f.mu.Lock()
	found := len(f.results)
	f.mu.Unlock()
	chords := g.findChords()
	counted := fmt.Sprintf("%d sessions found", found)
	if groups := len(f.groups(g)); groups > 0 {
		counted = plural(found, "session") + " and " + plural(groups, "group") + " found"
	}
	return fmt.Sprintf("%s, %d on the map   ↑ ↓ choose, enter opens, %s copies how to resume, %s shows it, esc clears",
		counted, g.scene.Matching(), chords.Copy, chords.Show)
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func (f *finder) keys(g *Game) bool {
	just := g.just
	if f.ime != nil {
		if handled, err := f.ime.Update(); err != nil {
			g.SetStatus(err.Error())
		} else if handled {
			f.follow(g, false)
			f.provisional(g)
			f.show(g)
			return true
		}
	}
	if f.opening && f.current() {
		return f.openAnswer(g)
	}
	f.show(g)
	h, chosen := f.chosen()
	switch {
	case just(ebiten.KeyEscape):
		f.end()
		g.filterOn("")
		return false
	case chosen && g.chordPressed(g.findChords().Copy):
		line := resumeLine(h)
		g.SetStatus("copied: " + line)
		g.act(city.Action{Kind: city.ActionCopy, Text: line})
		return true
	case chosen && g.chordPressed(g.findChords().Show):
		f.end()
		if !g.scene.Select(h.SessionID) {
			g.SetStatus("that session is not on the map")
		}
		return false
	case just(ebiten.KeyEnter), just(ebiten.KeyKPEnter):
		// Enter typed straight after the words, before the pause has
		// let the search run, searches at once and opens the answer
		// when it comes; the results on show may be an earlier query's.
		if f.field.query() != "" && !f.current() && g.search != nil {
			f.opening = true
			f.follow(g, true)
			if f.current() {
				return f.openAnswer(g)
			}
			return true
		}
		if chosen {
			return f.open(g, h)
		}
		if gr, ok := f.chosenGroup(g); ok {
			f.end()
			g.filterOn("")
			if !g.scene.FocusGroup(gr.ID) {
				g.SetStatus(gr.Name + " has no session on the map")
			}
			return false
		}
		f.end()
		return false
	case just(ebiten.KeyBackspace):
		f.field.backspace()
	case just(ebiten.KeyArrowDown):
		f.move(1, len(f.groups(g)))
	case just(ebiten.KeyArrowUp):
		f.move(-1, len(f.groups(g)))
	}
	if g.scriptedRune != 0 {
		f.field.insert(string(g.scriptedRune))
	}
	if s, ok := g.pasted(f); ok {
		f.field.insert(s)
	}
	f.follow(g, false)
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
	selected := f.selected
	f.mu.Unlock()
	in := ui.FinderInput{Query: shown, Caret: caret, Status: f.status(g), Words: strings.Fields(f.field.query()), Selected: selected,
		Results: f.rows(g, g.now())}
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
		Dir: h.Dir, Branch: h.Branch, Age: format.Age(now.Sub(h.At)), Gone: h.Gone, Packed: h.Packed,
		Recap: h.Recap, LastPrompt: h.LastPrompt,
	}
}

// filterOn sets the map's filter, which dims what does not match.
func (g *Game) filterOn(query string) {
	g.query = query
	g.scene.SetFilter(query)
}
