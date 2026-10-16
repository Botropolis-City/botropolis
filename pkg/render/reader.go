package render

import (
	"errors"
	"path/filepath"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// readerPage is how many turns the pane asks for before it has been
// drawn and knows how many fit.
const readerPage = 40

func (g *Game) reading() bool {
	_, ok := g.overlays.top().(*reader)
	return ok
}

// openReader puts the reading pane for the selection on the stack: o,
// or read in its summary. Item 83.
func (g *Game) openReader() {
	b := g.scene.Selected()
	if b == nil {
		g.SetStatus("select a building first")
		return
	}
	if g.reading() {
		return
	}
	r := &reader{id: b.Session.ID, count: readerPage}
	g.overlays.push(r)
	r.read(g, 0)
}

// reader is the pane that reads a session inside the city. It holds the
// page it shows and nothing else, and goes with it when it closes.
type reader struct {
	id string
	// back is the skips of the pages PgUp left, so PgDn retraces them.
	back  []int
	skip  int
	count int
	// shown is how many turns the last frame showed whole, which is how
	// far PgUp goes back.
	shown int
	// panel is where the last frame laid the pane out, guarded by g.mu.
	panel ui.ReaderPanel

	mu   sync.Mutex
	page claude.Conversation
	err  string
	// gone says the transcript has been deleted, and kept is what the
	// prompt history still has for the session.
	gone bool
	kept []claude.Prompt
}

func (r *reader) isGone() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.gone
}

// errTranscriptGone is a session whose transcript Claude Code has
// deleted, which the pane answers with the prompts its history kept.
var errTranscriptGone = errors.New("the transcript has been deleted")

// read asks for the page count turns back from skip, off the frame.
func (r *reader) read(g *Game, skip int) {
	r.skip = skip
	readConversation, readPrompts, dispatch := g.readConversation, g.readPrompts, g.dispatch
	home := g.home
	if readConversation == nil {
		readConversation = func(id string, skip, count int) (claude.Conversation, error) {
			path, ok := transcriptFor(home, id)
			if !ok {
				return claude.Conversation{}, errTranscriptGone
			}
			return claude.ReadConversation(path, skip, count)
		}
	}
	if readPrompts == nil {
		readPrompts = func(id string) ([]claude.Prompt, error) {
			history, err := claude.ReadPromptHistory(filepath.Join(home, ".claude", "history.jsonl"))
			return history[id], err
		}
	}
	if dispatch == nil {
		dispatch = func(run func()) { go run() }
	}
	id, count := r.id, r.count
	dispatch(func() {
		conv, err := readConversation(id, skip, count)
		var kept []claude.Prompt
		gone := errors.Is(err, errTranscriptGone)
		if gone {
			kept, err = readPrompts(id)
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		if skip != r.skip {
			return
		}
		r.page, r.gone, r.kept, r.err = conv, gone, kept, ""
		if err != nil {
			r.err = err.Error()
		}
	})
}

func (r *reader) keys(g *Game) bool {
	just := g.just
	switch {
	case just(ebiten.KeyEscape):
		return false
	case just(ebiten.KeyEnter), just(ebiten.KeyKPEnter):
		if r.isGone() {
			if b := g.scene.Selected(); b != nil {
				g.act(city.Action{Kind: city.ActionNew, Dir: b.Session.CWD})
			}
			return false
		}
		g.act(g.scene.Activate())
		return false
	case just(ebiten.KeyPageUp):
		r.mu.Lock()
		atStart := r.page.AtStart && r.shown >= len(r.page.Messages)
		r.mu.Unlock()
		if !atStart && r.shown > 0 {
			r.back = append(r.back, r.skip)
			r.read(g, r.skip+r.shown)
		}
	case just(ebiten.KeyPageDown):
		if n := len(r.back); n > 0 {
			skip := r.back[n-1]
			r.back = r.back[:n-1]
			r.read(g, skip)
		}
	}
	return true
}

// click closes the pane when the press lands outside it.
// A press on a kept prompt's copy button copies that prompt whole,
// through the clipboard seam the card's copy path uses.
func (r *reader) click(g *Game, at city.Point) bool {
	g.mu.Lock()
	panel, copies := g.readerRect, r.panel.Copies
	g.mu.Unlock()
	if !panel.Contains(at) {
		return false
	}
	for _, c := range copies {
		if !c.Button.Rect.Contains(at) {
			continue
		}
		r.mu.Lock()
		var text string
		if c.Turn < len(r.kept) {
			text = r.kept[c.Turn].Text
		}
		r.mu.Unlock()
		if text != "" {
			g.act(city.Action{Kind: city.ActionCopy, Text: text})
			g.SetStatus("copied the prompt")
		}
	}
	return true
}

// layout lays the pane out for a frame, and keeps where it went, for the
// clicks that come before the next one.
func (r *reader) layout(g *Game, th ui.Theme, width, height float64, measure ui.Measure) ui.ReaderPanel {
	p := ui.LayoutReader(th, width, height, r.input(g), measure)
	r.shown = p.Shown
	g.mu.Lock()
	g.readerRect, r.panel = p.Rect, p
	g.mu.Unlock()
	return p
}

// input is the page as the layout takes it. A pending call leads only
// when the session needs you: from the transcript alone, a command
// waiting on your approval and one still running look the same.
func (r *reader) input(g *Game) ui.ReaderInput {
	r.mu.Lock()
	page, err, gone, kept := r.page, r.err, r.gone, r.kept
	r.mu.Unlock()
	in := ui.ReaderInput{AtStart: page.AtStart, Err: err, Gone: gone}
	if gone {
		for _, p := range kept {
			in.Turns = append(in.Turns, ui.ReaderTurn{From: ui.FromYou, Text: p.Text})
		}
		if len(kept) == 0 {
			in.Note = "Claude Code kept no prompts for it"
		}
	}
	var session state.Session
	for _, b := range g.scene.City().Buildings() {
		if b.Session.ID == r.id {
			session = b.Session
			in.Title = b.Card(g.now()).Title
		}
	}
	for _, m := range page.Messages {
		in.Turns = append(in.Turns, ui.ReaderTurn{From: readerFrom(m.From), Text: m.Text})
	}
	for _, q := range page.Questions {
		in.Questions = append(in.Questions, ui.ReaderQuestion{Text: q.Text, Options: q.Options})
	}
	if session.State == state.NeedsYou && len(page.Questions) == 0 {
		for _, call := range page.Pending {
			in.Approval = append(in.Approval, call.Input)
		}
		if len(in.Approval) > 0 {
			in.Note = session.Note
		}
	}
	return in
}

func readerFrom(f claude.From) ui.ReaderFrom {
	switch f {
	case claude.FromClaude:
		return ui.FromClaude
	case claude.FromTool:
		return ui.FromTool
	}
	return ui.FromYou
}

func (r *reader) draw(g *Game, screen *ebiten.Image, width, height float64) {
	th := g.theme
	scrim := th.Palette.Panel
	scrim.A = 0x60
	g.roundRect(screen, city.RectAt(0, 0, width, height), 0, scrim)
	p := r.layout(g, th, width, height, g.faces.Measure)
	// Opaque, unlike a plate: a pane is read, and the card behind it
	// showed through the plates' translucency.
	fill := p.Fill
	fill.A = 0xff
	g.roundRect(screen, p.Rect, p.Radius, fill)
	g.text(screen, p.Title.At, p.Title.Text, p.Title.Size, th.Palette.Text)
	g.text(screen, p.Subtitle.At, p.Subtitle.Text, p.Subtitle.Size, th.Palette.Dim)
	for _, lines := range [][]ui.ReaderLine{p.Lead, p.Lines} {
		for _, l := range lines {
			g.text(screen, l.At, l.Text, l.Size, l.Color)
		}
	}
	if p.Start.Text != "" {
		g.text(screen, p.Start.At, p.Start.Text, p.Start.Size, th.Palette.Dim)
	}
	for _, c := range p.Copies {
		g.roundRect(screen, c.Button.Rect, th.Radius()/2, th.Palette.Hairline)
		g.run(screen, c.Button.Label, th.Palette.Accent)
	}
	g.text(screen, p.Keys.At, p.Keys.Text, p.Keys.Size, th.Palette.Dim)
}
