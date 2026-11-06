package render

import (
	"fmt"
	"image/color"
	"path/filepath"
	"slices"
	"sort"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/group"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// The tidy-up walk (item 333): a panel on the overlay stack with
// triage's shape, one card per decision. First Start clean, Walk through
// it or Later; then one card per group, strongest first; then one for
// the quiet sessions in no group, answered pack them up. Nothing is
// written until it is left: walking past the last card confirms it all,
// Later or esc before any choice writes nothing, and esc after one keeps
// only what was decided.

const (
	cardStart = "start"
	cardQuiet = "quiet"
	// cardTime is what a card takes, for the title's estimate.
	cardTime = 15 * time.Second
)

type walk struct {
	review group.Review
	// cards are the card keys in order: start, a group's id, quiet.
	cards []string
	at    int
	// member is the chosen member on a group's card.
	member int
	// quietFor is the quiet card's threshold, quiet its sessions,
	// answered whether the card was passed, pack its answer, and
	// districts whether show me is open.
	quietFor  time.Duration
	quiet     []state.Session
	answered  bool
	pack      bool
	districts bool
	// journal is whether the journal follows the walk however it is
	// left: it opened in the journal's place, after a long return.
	journal bool
	rect    city.Rect
	buttons []walkButton
}

type walkButton struct {
	label string
	key   ebiten.Key
	rect  city.Rect
}

func (g *Game) walk() *walk {
	w, _ := g.overlays.top().(*walk)
	return w
}

// openWalk puts the walk on the stack when there is something to tidy,
// and reports whether it did.
func (g *Game) openWalk() bool {
	if g.walk() != nil {
		return true
	}
	w := &walk{quietFor: g.quietDays(), pack: true}
	w.review = reviewOf(g.walkGroups())
	w.cards = []string{cardStart}
	for _, row := range w.review.Rows {
		w.cards = append(w.cards, row.ID)
	}
	w.quiet = g.quietSessions(w.quietFor)
	if len(w.quiet) > 0 {
		w.cards = append(w.cards, cardQuiet)
	}
	if len(w.cards) == 1 {
		return false
	}
	g.overlays.push(w)
	g.scene.ClearFocus()
	return true
}

// looking is whether someone is watching the city, rather than a
// screenshot, a recording, a share or a driven run, which the walk never
// opens over by itself.
func (g *Game) looking() bool {
	return g.share == nil && g.screenshot == "" && g.record == "" && !g.driven
}

// tidyAfterGap opens the walk in the journal's place after a gap longer
// than tidy_days, when there is something to tidy, and reports whether
// it did; the journal follows it.
func (g *Game) tidyAfterGap() bool {
	if !g.looking() || g.awaySince.IsZero() || g.tidyAfter <= 0 || g.scene.Now().Sub(g.awaySince) < g.tidyAfter {
		return false
	}
	if !g.openWalk() {
		return false
	}
	g.walk().journal = true
	g.firstRunAsked = true
	return true
}

// tidyOffer is the journal strip's offer of the walk, when there is
// something to tidy: how many sessions went quiet while you were away,
// or how many groups no walk has settled.
func (g *Game) tidyOffer(since time.Time) string {
	quietFor := g.quietDays()
	went := 0
	for _, s := range g.quietSessions(quietFor) {
		if s.LastActivity.After(since.Add(-quietFor)) {
			went++
		}
	}
	if went > 0 {
		return fmt.Sprintf("%s went quiet while you were away — tidy up?  t", plural(went, "session"))
	}
	if n := len(g.walkGroups()); n > 0 {
		return fmt.Sprintf("%s the city found are waiting — tidy up?  t", plural(n, "group"))
	}
	return ""
}

// walkGroups are the groups the walk asks about, strongest first: those
// with a session at work, then the larger, then those held by more sure
// ties. A pair joined by a likely tie is never asked about; it stays a
// guess on the map. Nor is a group with one session on the map (item
// 344): it is on the board, in the picker and on its card, but a card
// here has to earn its place, and a question about one building does not.
func (g *Game) walkGroups() []state.Group {
	byID := map[string]state.Session{}
	for _, b := range g.scene.City().Buildings() {
		byID[b.Session.ID] = b.Session
	}
	type weighed struct {
		g                state.Group
		live, size, sure int
	}
	var ws []weighed
	for _, gr := range g.scene.Groups() {
		if gr.Reviewed {
			continue
		}
		w := weighed{g: gr, size: len(gr.Members)}
		for _, id := range gr.Members {
			s := byID[id]
			if s.State != state.Resting && s.ID != "" {
				w.live++
			}
			if !s.GroupLikely {
				w.sure++
			}
		}
		if w.size < 2 || (w.size == 2 && w.sure < 2) {
			continue
		}
		ws = append(ws, w)
	}
	sort.SliceStable(ws, func(i, j int) bool {
		a, b := ws[i], ws[j]
		if a.live != b.live {
			return a.live > b.live
		}
		if a.size != b.size {
			return a.size > b.size
		}
		return a.sure > b.sure
	})
	out := make([]state.Group, 0, len(ws))
	for _, w := range ws {
		out = append(out, w.g)
	}
	return out
}

// quietSessions are the sessions in no group the map shows, quiet for
// at least d.
func (g *Game) quietSessions(d time.Duration) []state.Session {
	var all []state.Session
	for _, b := range g.scene.City().Buildings() {
		all = append(all, b.Session)
	}
	ids := state.Quiet(all, g.scene.Now(), d)
	out := make([]state.Session, 0, len(ids))
	for _, s := range all {
		if slices.Contains(ids, s.ID) {
			out = append(out, s)
		}
	}
	return out
}

func (w *walk) cardKeys() []string { return w.cards }

func (w *walk) card() string { return w.cards[w.at] }

func (w *walk) title() string {
	minutes := int((time.Duration(len(w.cards))*cardTime + time.Minute - 1) / time.Minute)
	return fmt.Sprintf("Tidy up · %d of %d · about %s", w.at+1, len(w.cards), plural(minutes, "minute"))
}

// row is the group card's row, or -1 when the card is not a group's or
// its row went in a split.
func (w *walk) row() int {
	return slices.IndexFunc(w.review.Rows, func(r group.ReviewRow) bool { return r.ID == w.card() })
}

func (w *walk) memberTitles() []string {
	i := w.row()
	if i < 0 {
		return nil
	}
	var titles []string
	for _, m := range w.review.Rows[i].Members {
		titles = append(titles, m.Title)
	}
	return titles
}

func (w *walk) buttonDefs() []walkButton {
	switch w.card() {
	case cardStart:
		return []walkButton{{label: "c  Start clean", key: ebiten.KeyC}, {label: "enter  Walk through it", key: ebiten.KeyEnter}, {label: "l  Later", key: ebiten.KeyL}}
	case cardQuiet:
		return []walkButton{{label: "enter  pack them up", key: ebiten.KeyEnter}, {label: "x  leave them", key: ebiten.KeyX},
			{label: "v  show me", key: ebiten.KeyV}, {label: "[ ]  threshold", key: ebiten.KeyBracketRight}, {label: "p  back", key: ebiten.KeyP}}
	}
	return []walkButton{{label: "enter  keep", key: ebiten.KeyEnter}, {label: "r  rename", key: ebiten.KeyR}, {label: "x  drop", key: ebiten.KeyX},
		{label: "s  split", key: ebiten.KeyS}, {label: "-  take one out", key: ebiten.KeyMinus}, {label: "n  next", key: ebiten.KeyN}, {label: "p  back", key: ebiten.KeyP}}
}

func (w *walk) buttonLabels() []string {
	var labels []string
	for _, b := range w.buttonDefs() {
		labels = append(labels, b.label)
	}
	return labels
}

func (w *walk) quietIDs() []string {
	ids := make([]string, 0, len(w.quiet))
	for _, s := range w.quiet {
		ids = append(ids, s.ID)
	}
	return ids
}

func (w *walk) quietDistricts() []string {
	var names []string
	for _, s := range w.quiet {
		if name := filepath.Base(s.CWD); !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func (w *walk) quietLine() string {
	dirs := map[string]bool{}
	for _, s := range w.quiet {
		dirs[s.CWD] = true
	}
	return fmt.Sprintf("%s in %s have been quiet for more than %s and belong to no group — pack them up?",
		plural(len(w.quiet), "session"), directories(len(dirs)), daysPhrase(w.quietFor))
}

func directories(n int) string {
	if n == 1 {
		return "1 directory"
	}
	return fmt.Sprintf("%d directories", n)
}

// show points the camera at the card: a group's card focuses its group,
// the rest show the whole city.
func (w *walk) show(g *Game) {
	if w.row() >= 0 {
		g.scene.FocusGroup(w.card())
		return
	}
	g.scene.ClearFocus()
	g.scene.Fit()
}

func (w *walk) step(g *Game, by int) {
	w.at = max(0, min(w.at+by, len(w.cards)-1))
	w.member = 0
	w.show(g)
}

// next moves on, or, past the last card, confirms the walk.
func (w *walk) next(g *Game) {
	if w.at == len(w.cards)-1 {
		w.confirm(g)
		return
	}
	w.step(g, 1)
}

func (w *walk) keys(g *Game) bool {
	for _, b := range w.buttonDefs() {
		if b.key == ebiten.KeyBracketRight {
			continue
		}
		if g.just(b.key) {
			return w.press(g, b.key)
		}
	}
	switch {
	case g.just(ebiten.KeyKPEnter):
		return w.press(g, ebiten.KeyEnter)
	case g.just(ebiten.KeyEscape):
		if w.review.AnyDecided() || w.answered {
			w.doneForNow(g)
			return true
		}
		g.Notify("tidy up later: nothing was changed", ui.ToneNone, "")
		w.leave(g, w.journal)
	case g.just(ebiten.KeyJ):
		w.member++
		if n := len(w.memberTitles()); w.member >= n {
			w.member = max(0, n-1)
		}
	case g.just(ebiten.KeyK):
		w.member = max(0, w.member-1)
	case w.card() == cardQuiet && g.just(ebiten.KeyBracketLeft):
		w.turn(g, -1)
	case w.card() == cardQuiet && g.just(ebiten.KeyBracketRight):
		w.turn(g, 1)
	}
	return true
}

// press does what a card's key does, and reports whether the walk stays
// open.
func (w *walk) press(g *Game, key ebiten.Key) bool {
	switch w.card() {
	case cardStart:
		switch key {
		case ebiten.KeyC:
			w.startClean(g)
		case ebiten.KeyEnter:
			w.step(g, 1)
		case ebiten.KeyL:
			g.Notify("tidy up later: nothing was changed", ui.ToneNone, "")
			w.leave(g, w.journal)
		}
		return true
	case cardQuiet:
		switch key {
		case ebiten.KeyEnter:
			w.answered, w.pack = true, true
			w.next(g)
		case ebiten.KeyX:
			w.answered, w.pack = true, false
			w.next(g)
		case ebiten.KeyV:
			w.districts = !w.districts
		case ebiten.KeyBracketRight:
			w.turn(g, 1)
		case ebiten.KeyP:
			w.step(g, -1)
		}
		return true
	}
	i := w.row()
	switch key {
	case ebiten.KeyEnter:
		w.review.Decide(i)
		w.next(g)
	case ebiten.KeyX:
		if i >= 0 && w.review.Rows[i].Keep {
			w.review.Toggle(i)
		}
		w.next(g)
	case ebiten.KeyS:
		w.review.Split(i)
		w.next(g)
	case ebiten.KeyR:
		if i >= 0 {
			row := w.review.Rows[i]
			g.openRenamerFor(row.ID, row.Name, func(name string) { w.review.Rename(w.row(), name) })
		}
	case ebiten.KeyMinus:
		if i >= 0 && w.member < len(w.review.Rows[i].Members) {
			w.review.TakeOut(i, w.review.Rows[i].Members[w.member].Session)
			w.member = max(0, min(w.member, len(w.review.Rows[i].Members)-1))
		}
	case ebiten.KeyN:
		w.next(g)
	case ebiten.KeyP:
		w.step(g, -1)
	}
	return true
}

// turn moves the quiet card's threshold by a day, never under one.
func (w *walk) turn(g *Game, days int) {
	w.quietFor = max(24*time.Hour, w.quietFor+time.Duration(days)*24*time.Hour)
	w.quiet = g.quietSessions(w.quietFor)
}

// confirm writes everything on the cards, packs the quiet sessions up
// when answered so, and leaves for the journal.
func (w *walk) confirm(g *Game) {
	review := w.review
	g.act(city.Action{Kind: city.ActionReviewGroups, Review: &review})
	if w.answered && w.pack {
		g.sweepIDs(w.quietIDs(), w.quietFor)
	}
	g.Notify("tidied up", ui.ToneNone, "")
	w.leave(g, true)
}

// doneForNow keeps only what was decided, and leaves.
func (w *walk) doneForNow(g *Game) {
	review := w.review.DecidedOnly()
	g.act(city.Action{Kind: city.ActionReviewGroups, Review: &review})
	if w.answered && w.pack {
		g.sweepIDs(w.quietIDs(), w.quietFor)
	}
	g.Notify("done for now: what you decided is kept, the rest stays as guesses", ui.ToneNone, "")
	w.leave(g, w.journal)
}

// startClean packs up everything in no group that was resting before
// you left, or, with no gap to go by, everything quiet, then confirms
// every group the walk would ask about, in one step.
func (w *walk) startClean(g *Game) {
	ids := w.quietIDs()
	if gap := g.awaySince; !gap.IsZero() && g.scene.Now().Sub(gap) >= time.Duration(max(1, g.journalHours))*time.Hour {
		ids = ids[:0:0]
		for _, b := range g.scene.City().Buildings() {
			s := b.Session
			if s.Group == "" && s.State == state.Resting && !s.Alive && !s.Packed && s.LastActivity.Before(gap) {
				ids = append(ids, s.ID)
			}
		}
	}
	review := w.review
	g.sweepIDs(ids, w.quietFor)
	g.act(city.Action{Kind: city.ActionReviewGroups, Review: &review})
	w.leave(g, true)
}

// leave takes the walk off the stack, the map out of focus, and opens
// the journal after a confirm.
func (w *walk) leave(g *Game, journal bool) {
	g.overlays.remove(w)
	g.scene.ClearFocus()
	if journal {
		g.openJournal()
	}
}

func (w *walk) click(g *Game, at city.Point) bool {
	g.mu.Lock()
	buttons := slices.Clone(w.buttons)
	g.mu.Unlock()
	for _, b := range buttons {
		if b.rect.Contains(at) {
			w.press(g, b.key)
			return true
		}
	}
	return true
}

// layout places the panel and its buttons along its foot.
func (w *walk) layout(th ui.Theme, width, height float64, measure ui.Measure) {
	grid := th.Grid()
	_, title := measure("", ui.Title)
	_, body := measure("", ui.Body)
	_, small := measure("", ui.Small)
	lines := max(3, len(w.memberTitles())+2)
	if w.card() == cardQuiet && w.districts {
		lines += len(w.quietDistricts())
	}
	lines = min(lines, 14)
	panelWidth := min(width-grid*8, grid*100)
	panelHeight := grid*5 + title + float64(lines)*(body+grid/4) + small + grid*2
	w.rect = city.RectAt((width-panelWidth)/2, height-panelHeight-grid*8, panelWidth, panelHeight)
	x, y := w.rect.Min.X+grid*2, w.rect.Max.Y-grid*2-small-grid/2
	w.buttons = w.buttons[:0]
	for _, b := range w.buttonDefs() {
		bw, _ := measure(b.label, ui.Small)
		if x+bw+grid*2 > w.rect.Max.X-grid*2 {
			break
		}
		b.rect = city.RectAt(x, y, bw+grid*2, small+grid/2)
		w.buttons = append(w.buttons, b)
		x += bw + grid*3
	}
}

func (w *walk) draw(g *Game, screen *ebiten.Image, width, height float64) {
	th := g.theme
	g.mu.Lock()
	w.layout(th, width, height, g.faces.Measure)
	g.mu.Unlock()
	grid := th.Grid()
	_, title := g.faces.Measure("", ui.Title)
	_, body := g.faces.Measure("", ui.Body)
	g.roundRect(screen, w.rect, th.Radius(), th.Palette.Panel)
	inner := w.rect.Width() - grid*4
	x, y := w.rect.Min.X+grid*2, w.rect.Min.Y+grid*2
	g.text(screen, city.Point{X: x, Y: y}, g.clip(w.title(), inner, ui.Title), ui.Title, th.Palette.Text)
	y += title + grid
	line := func(s string, ink color.NRGBA) {
		if y+body > w.rect.Max.Y-grid*4 {
			return
		}
		g.text(screen, city.Point{X: x, Y: y}, g.clip(s, inner, ui.Body), ui.Body, ink)
		y += body + grid/4
	}
	switch w.card() {
	case cardStart:
		line("Start clean packs up what was quiet before you left and keeps every group the city found.", th.Palette.Text)
		line("Walk through it asks about each group, then the quiet sessions.", th.Palette.Dim)
	case cardQuiet:
		line(w.quietLine(), th.Palette.Text)
		if w.districts {
			for _, d := range w.quietDistricts() {
				line("   "+d, th.Palette.Dim)
			}
		}
	default:
		if i := w.row(); i >= 0 {
			row := w.review.Rows[i]
			line(fmt.Sprintf("%s · %s", row.Name, plural(len(row.Members), "session")), th.Palette.Text)
			line(row.Reason, th.Palette.Dim)
			for k, t := range w.memberTitles() {
				mark := "   "
				if k == w.member {
					mark = " › "
				}
				line(mark+t, th.Palette.Text)
			}
		}
	}
	g.mu.Lock()
	buttons := slices.Clone(w.buttons)
	g.mu.Unlock()
	for _, b := range buttons {
		g.roundRect(screen, b.rect, th.Radius()/2, th.Palette.Hairline)
		g.text(screen, city.Point{X: b.rect.Min.X + grid, Y: b.rect.Min.Y + grid/4}, b.label, ui.Small, th.Palette.Accent)
	}
}
