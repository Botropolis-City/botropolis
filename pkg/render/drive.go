package render

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/find"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// Drive runs the city a tick at a time with no window, for the tests that
// measure a task in the inputs it takes, the ticks until its result is
// drawn and the work done on the way. Ebitengine's clock never runs: each
// Tick is one Update and one Draw, and the city's clock counts ticks, so
// the same inputs take the same ticks on an idle machine and a loaded
// one. Item 91.
type Drive struct {
	game   *Game
	screen *ebiten.Image
	queue  []driveInput
	steps  int
	// sent is the tick the last input began on, and began the tick the
	// input being delivered began on.
	sent  uint64
	began uint64
	acted []city.Action
	// actedAt is the tick each action was handed over on.
	actedAt []uint64
}

type DriveOptions struct {
	// Home is the directory holding the .claude the city is drawn from.
	Home string
	// Width and Height are the window's size; zero is the default.
	Width, Height int
	// Now is the moment the city is drawn at, and its clock starts from;
	// zero is the wall clock's now.
	Now time.Time
	// Seen is when the map was last seen, as its layout would remember;
	// zero is a map never opened before. A Seen journal_hours or more
	// before Now opens the journal (item 86).
	Seen time.Time
	// Events is the event log the city opens with, as the daemon would
	// hand it over (item 100).
	Events []events.Event
}

// Shown is what the last frame drew.
type Shown struct {
	Help, Views, Settings, Breakdown, Timeline, Search, Sidebar bool
	// Journal is the journal's entries when it is drawn, each with what
	// it was doing and what is next (item 86).
	Journal []city.JournalEntry
	// Selected is the session whose card or sidebar entry was drawn as
	// the selection, or "" for none.
	Selected string
	// Buildings is the sessions drawn as buildings, by id, sorted.
	Buildings []string
	// Lit is the buildings the filter leaves undimmed, by id, sorted,
	// when there is a filter; nil when there is none.
	Lit []string
	// Lens and View name the lens the map is in and its view, and
	// Captions the kinds explain mode or a first sighting captioned
	// (item 88).
	Lens, View string
	Captions   []string
	// Explaining is whether explain mode's key is held.
	Explaining bool
	// Triage is the session triage is on, or "" when it is not walking,
	// and Stream whether the stream's column is up.
	Triage string
	Stream bool
	// StreamRect is where the stream was drawn, StreamScrolled how many
	// of its newest entries are scrolled past, and Zoom the camera's
	// (item 100).
	StreamRect     city.Rect
	StreamScrolled int
	Zoom           float64
	// KeyListRect is where the reference's key list was drawn, empty
	// when it was not, KeyListScrolled how many of its rows are scrolled
	// past, and KeyListMax the most they can be (item 127).
	KeyListRect                 city.Rect
	KeyListScrolled, KeyListMax int
	// SidebarRect is where the sidebar was drawn, empty when it was not,
	// SidebarScrolled how many of its rows are scrolled past, SidebarMax
	// the most they can be, and SidebarSessions the sessions whose rows
	// it drew (item 136). Camera is the camera's offset.
	SidebarRect                 city.Rect
	SidebarScrolled, SidebarMax int
	SidebarSessions             []string
	Camera                      city.Point
	// SidebarRows is how many rows the sidebar drew, CardRect where the
	// selection's card was drawn, and LensRect where the lens panel was,
	// each empty when it was not (item 140).
	SidebarRows        int
	CardRect, LensRect city.Rect
	// StripBottom is where the strip along the top ends, and FooterTop
	// where the footer begins, needs-you line and all.
	StripBottom, FooterTop float64
	// Found is the finder's results, by session id, best first, while it
	// is open.
	Found []string
	// Waiting is the footer's needs-you line and WaitingRect where it was
	// drawn; empty when nothing waits.
	Waiting     string
	WaitingRect city.Rect
	// WaitingHovered is whether the line was drawn hovered.
	WaitingHovered bool
	// Settled is whether the frame was drawn with nothing an input set
	// going still on its way: a panel that slides or a camera that eases
	// is not shown until it arrives.
	Settled bool
	// Labels is every badge, station label and district sign the frame
	// drew, where it drew it (item 102).
	Labels []ShownLabel
	// WorldDrawn is whether the map drew any world sprite, TagsDrawn
	// how many road tags it drew, and TagsUnderWorld how many of those
	// it drew before a world sprite, where the sprite could cover them
	// (item 75).
	WorldDrawn                bool
	TagsDrawn, TagsUnderWorld int
	// Sprites is how many world sprites the frame drew, and at night
	// WindowDraws how many buildings' windows it lit and Punches how many
	// sprites it cut out of the lights behind them (item 166).
	Sprites, WindowDraws, Punches int
}

// ShownLabel is one drawn label: its kind, "sign", "plate", "caption",
// "station label", "badge" or "tag", where it stands, and whether it
// was drawn faded under a highlight.
type ShownLabel struct {
	Kind  string
	Rect  city.Rect
	Faded bool
}

// driveInput is one tick's share of an input: a key, or one half of a
// click.
type driveInput struct {
	key            string
	pointer        string
	at             *city.Point
	press, release bool
	wheel          float64
}

// keys the harness refuses: each reaches outside the city, p to the
// Pictures directory and u to a Claude Code probe.
var undriven = map[string]string{"p": "saves a file in the home directory", "u": "starts a Claude Code probe"}

// NewDrive builds the city on a home and draws its first frame.
func NewDrive(opts DriveOptions) (*Drive, error) {
	width, height := opts.Width, opts.Height
	if width <= 0 || height <= 0 {
		width, height = DefaultWidth, DefaultHeight
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	layout := city.NewLayout()
	layout.Seen = opts.Seen
	scene := city.NewScene(layout)
	scene.SetProjection(city.Isometric)
	theme := ui.NewTheme(1)
	scene.Resize(float64(width), float64(height))
	faces, err := newFaces(theme)
	if err != nil {
		return nil, err
	}
	art, err := driveArt()
	if err != nil {
		return nil, err
	}
	sprites, kitSprites := art.sprites, art.kits
	d := &Drive{screen: ebiten.NewImage(width, height)}
	scene.AddEvents(opts.Events)
	g := NewGame(scene, d, theme, faces, nil, sprites)
	g.kits = kitSprites
	g.home = opts.Home
	g.driven = true
	g.journalHours = config.DefaultJournalHours
	// No window to type into: typing reaches the finder the way --keys
	// does.
	g.ime = false
	g.started = now
	scene.SetClock(g.now, time.Local)
	g.summarise = func(context.Context, string) (string, error) {
		return "", errors.New("summaries are not generated while driven")
	}
	// Work handed off the frame runs on the spot, so a tick count never
	// depends on the scheduler.
	g.dispatch = func(run func()) { run() }
	d.game = g
	snapshot, err := driveSnapshot(opts.Home, now)
	if err != nil {
		return nil, err
	}
	// The finder searches an index built as the map opens, as the
	// daemon's is, and refreshed on each search.
	index := find.New(opts.Home)
	if err := index.Refresh(); err != nil {
		return nil, err
	}
	live := map[string]state.State{}
	for _, s := range snapshot.Sessions {
		live[s.ID] = s.State
	}
	g.search = func(q find.Query) ([]find.Hit, error) {
		if err := index.Refresh(); err != nil {
			return nil, err
		}
		q.Live = live
		return index.Find(q), nil
	}
	g.Offer(snapshot)
	g.AddEvents(nil)
	if err := d.Tick(); err != nil {
		return nil, err
	}
	return d, nil
}

// driveArt is the sprites and kits, decoded once for every city driven in
// a process: they are only read once loaded, and decoding the kits is
// most of what opening a driven city costs.
var driveArt = sync.OnceValues(func() (struct {
	sprites *sprites
	kits    *kits
}, error) {
	var art struct {
		sprites *sprites
		kits    *kits
	}
	var err error
	if art.sprites, err = loadSprites(); err != nil {
		return art, err
	}
	art.kits, err = loadKits()
	return art, err
})

// Do is the actor: the city's actions are recorded, never carried out.
func (d *Drive) Do(action city.Action) error {
	d.acted = append(d.acted, action)
	d.actedAt = append(d.actedAt, d.game.ticks)
	return nil
}

// Acted is every action the city asked for, in order.
func (d *Drive) Acted() []city.Action { return d.acted }

// Press queues a key for the next tick the way --keys names one: a key
// by its name, "ctrl+" and a key, or a typed character. It is one step.
func (d *Drive) Press(name string) error {
	if why, ok := undriven[name]; ok {
		return fmt.Errorf("the harness does not press %s: it %s", name, why)
	}
	key := name
	for trimmed := true; trimmed; {
		trimmed = false
		for _, prefix := range []string{"ctrl+", "alt+", "cmd+", "shift+", "hold+", "release+"} {
			if rest, ok := strings.CutPrefix(key, prefix); ok {
				key, trimmed = rest, true
			}
		}
	}
	if _, _, ok := scriptedKey(key); !ok {
		return fmt.Errorf("no key called %q", name)
	}
	d.queue = append(d.queue, driveInput{key: name})
	d.steps++
	return nil
}

// Type queues text, a character a tick, as typing into a panel. Each
// word in it is one step, as the bar's targets count them; a prompt is
// one step whatever its length, so it goes through Prompt instead.
func (d *Drive) Type(text string) error {
	if err := d.typed(text); err != nil {
		return err
	}
	d.steps += len(strings.Fields(text))
	return nil
}

// Prompt queues a prompt, typed a character a tick, and the Enter that
// sends it. Typing a prompt and sending it is one act, so it is one step.
func (d *Drive) Prompt(text string) error {
	if err := d.typed(text); err != nil {
		return err
	}
	d.queue = append(d.queue, driveInput{key: "Enter"})
	d.steps++
	return nil
}

func (d *Drive) typed(text string) error {
	if strings.TrimSpace(text) == "" {
		return errors.New("nothing to type")
	}
	for _, r := range text {
		if !unicode.IsPrint(r) {
			return fmt.Errorf("%q cannot be typed", r)
		}
		d.queue = append(d.queue, driveInput{key: string(r)})
	}
	return nil
}

// Click queues a click on a target, as a scripted pointer names one: a
// session by title or id, "district:<name>" or "plant". It is one step,
// pressed on one tick and released on the next.
func (d *Drive) Click(target string) error {
	if _, ok := d.game.resolveTarget(target); !ok {
		return fmt.Errorf("nothing on the map called %q", target)
	}
	d.queue = append(d.queue, driveInput{pointer: target, press: true}, driveInput{pointer: target, release: true})
	d.steps++
	return nil
}

// ClickAt queues a click on a point of the window, whatever is there.
// It is one step.
func (d *Drive) ClickAt(at city.Point) error {
	d.queue = append(d.queue, driveInput{at: &at, press: true}, driveInput{at: &at, release: true})
	d.steps++
	return nil
}

// WheelAt turns the scroll wheel by notches with the pointer at a point
// of the window; up is positive. It is one step.
func (d *Drive) WheelAt(at city.Point, notches float64) error {
	d.queue = append(d.queue, driveInput{at: &at, wheel: notches})
	d.steps++
	return nil
}

// PointAt parks the pointer over a target, as --hover does, until it is
// moved again. Moving the pointer presses nothing, so it is not a step.
func (d *Drive) PointAt(target string) error {
	at, ok := d.game.resolveTarget(target)
	if !ok {
		return fmt.Errorf("nothing on the map called %q", target)
	}
	return d.PointAtPoint(at)
}

// PointAtPoint parks the pointer at a point of the window.
func (d *Drive) PointAtPoint(at city.Point) error {
	d.game.hover, d.game.hoverSet = at, true
	return nil
}

// Feed loads a home and hands the city its snapshot, as the feed does
// when sessions change; the ticks Until counts start from the tick that
// takes it. The finder keeps the index it opened with.
func (d *Drive) Feed(home string) error {
	snapshot, err := driveSnapshot(home, d.game.now())
	if err != nil {
		return err
	}
	d.game.Offer(snapshot)
	d.sent = d.game.ticks + 1
	return nil
}

// Steps is how many inputs have been sent.
func (d *Drive) Steps() int { return d.steps }

// Ticks is how many ticks the city has run, the one that drew its first
// frame included.
func (d *Drive) Ticks() int { return int(d.game.ticks) }

// Tick runs one Update and one Draw, delivering the next queued input.
func (d *Drive) Tick() error {
	g := d.game
	g.inject = nil
	if len(d.queue) > 0 {
		in := d.queue[0]
		d.queue = d.queue[1:]
		g.inject = &in
		// An input begins on its key or its press: a click's release
		// is the same input, and what a press does counts from the press.
		if !in.release {
			d.began = g.ticks + 1
		}
		if len(d.queue) == 0 {
			d.sent = d.began
		}
	}
	if err := g.Update(); err != nil {
		return err
	}
	g.Draw(d.screen)
	dropDeferred()
	return nil
}

// Run delivers what is queued and then runs n more ticks.
func (d *Drive) Run(n int) error {
	for len(d.queue) > 0 {
		if err := d.Tick(); err != nil {
			return err
		}
	}
	for range n {
		if err := d.Tick(); err != nil {
			return err
		}
	}
	return nil
}

// Until delivers what is queued and ticks until a settled frame shows
// what want asks for, returning how many ticks that took counted from the one that
// delivered the last input. It gives up after limit ticks past the last
// input.
func (d *Drive) Until(want func(Shown) bool, limit int) (int, error) {
	for len(d.queue) > 0 {
		if err := d.Tick(); err != nil {
			return 0, err
		}
	}
	shows := func() bool { s := d.Shown(); return s.Settled && want(s) }
	if d.sent > 0 && d.game.ticks >= d.sent && shows() {
		return int(d.game.ticks - d.sent + 1), nil
	}
	for range limit {
		if err := d.Tick(); err != nil {
			return 0, err
		}
		if shows() {
			return int(d.game.ticks - d.sent + 1), nil
		}
	}
	return 0, fmt.Errorf("not shown within %d ticks of the last input", limit)
}

// UntilActed delivers what is queued and ticks until the city hands
// the actor an action that want accepts, returning how many ticks that
// took counted from the one that delivered the last input.
func (d *Drive) UntilActed(want func(city.Action) bool, limit int) (int, error) {
	for len(d.queue) > 0 {
		if err := d.Tick(); err != nil {
			return 0, err
		}
	}
	for i := 0; ; i++ {
		for j, a := range d.acted {
			if d.actedAt[j] >= d.sent && want(a) {
				return int(d.actedAt[j] - d.sent + 1), nil
			}
		}
		if i == limit {
			return 0, fmt.Errorf("no such action within %d ticks of the last input", limit)
		}
		if err := d.Tick(); err != nil {
			return 0, err
		}
	}
}

// Shown is what the last frame drew.
func (d *Drive) Shown() Shown {
	s := d.game.shown
	s.Buildings = slices.Clone(s.Buildings)
	s.Lit = slices.Clone(s.Lit)
	return s
}

// Title is a session's title as the city has it, or "" for none.
func (d *Drive) Title(id string) string {
	return d.session(id).Title
}

// Dir is a session's working directory as the city has it.
func (d *Drive) Dir(id string) string {
	return d.session(id).CWD
}

// State is a session's state as the city has it, by name.
func (d *Drive) State(id string) string {
	return string(d.session(id).State)
}

func (d *Drive) session(id string) state.Session {
	for _, b := range d.game.scene.City().Buildings() {
		if b.Session.ID == id {
			return b.Session
		}
	}
	return state.Session{}
}

// Sessions is every session in the city, by id, sorted.
func (d *Drive) Sessions() []string {
	var ids []string
	for _, b := range d.game.scene.City().Buildings() {
		ids = append(ids, b.Session.ID)
	}
	slices.Sort(ids)
	return ids
}

// inject applies a driven input to this tick, the way --keys and a
// clip's script do.
func (g *Game) injectInput() {
	in := g.inject
	if in == nil {
		return
	}
	if in.key != "" {
		g.pressScripted(in.key)
	}
	if in.pointer != "" || in.at != nil {
		if in.at != nil {
			g.play.shown, g.play.pointer = true, *in.at
		} else if at, ok := g.resolveTarget(in.pointer); ok {
			g.play.shown, g.play.pointer = true, at
		}
		g.play.press, g.play.release = in.press, in.release
		g.play.wheel = in.wheel
	}
}

// noteShown records what this frame drew.
func (g *Game) noteShown() {
	s := Shown{
		Help: g.referenceOpen() != nil, Views: g.viewKey, Settings: g.settingsOpen, Breakdown: g.breakdown,
		Timeline: g.timeline, Search: g.finding() || g.query != "", Sidebar: g.sidebar,
	}
	if g.hidden {
		s = Shown{}
	}
	s.Lens, s.View = g.scene.Lens().Name(), g.scene.View().Name()
	s.Explaining = g.explainHeld()
	s.Stream = g.streamShown()
	g.mu.Lock()
	s.StreamRect, s.StreamScrolled = g.streamLaid.Rect, g.streamLaid.Skip
	g.mu.Unlock()
	s.Zoom = g.scene.Camera().Zoom
	s.Camera = g.scene.Camera().Offset
	if g.sidebar && !g.hidden {
		g.mu.Lock()
		sb := g.sidebarLayout
		g.mu.Unlock()
		s.SidebarRect, s.SidebarScrolled, s.SidebarMax = sb.Rect, sb.Skip, sb.Max
		s.SidebarRows = len(sb.Rows)
		for _, r := range sb.Rows {
			if r.Kind == ui.RowSession {
				s.SidebarSessions = append(s.SidebarSessions, r.ID)
			}
		}
	}
	if step, ok := g.scene.TriageStep(); ok && g.triage() != nil {
		s.Triage = step.Building.Session.ID
	}
	for _, c := range g.captions {
		s.Captions = append(s.Captions, c.Kind.Name())
	}
	s.Settled = !g.scene.Easing()
	g.mu.Lock()
	s.Labels = slices.Clone(g.shownLabels)
	s.WorldDrawn, s.TagsDrawn, s.TagsUnderWorld = g.drawnOrder.lastWorld > 0, len(g.drawnOrder.tags), g.drawnOrder.tagsUnderWorld()
	s.Sprites, s.WindowDraws, s.Punches = g.drawnOrder.n-len(g.drawnOrder.tags), g.windowDraws, g.punches
	g.mu.Unlock()
	if b := g.scene.Selected(); b != nil && !g.hidden && (g.sidebar || g.pinned.Rect.Area() > 0) {
		s.Selected = b.Session.ID
	}
	if !g.hidden {
		s.CardRect = g.pinned.Rect
		for _, o := range g.overlays {
			if p, ok := o.(*lensPanel); ok {
				s.LensRect = p.laid.Rect
			}
		}
	}
	g.mu.Lock()
	if !g.hidden {
		s.Waiting, s.WaitingRect, s.WaitingHovered = g.footerBar.Waiting.Text, g.footerBar.WaitingRect, g.waitingHovered
	}
	for _, h := range g.hits {
		if h.hit.Building != nil && !slices.Contains(s.Buildings, h.hit.Building.Session.ID) {
			s.Buildings = append(s.Buildings, h.hit.Building.Session.ID)
		}
	}
	g.mu.Unlock()
	slices.Sort(s.Buildings)
	if g.query != "" && !g.hidden {
		s.Lit = []string{}
		for _, b := range g.scene.City().Buildings() {
			if !g.scene.Dimmed(b) {
				s.Lit = append(s.Lit, b.Session.ID)
			}
		}
		slices.Sort(s.Lit)
	}
	s.StripBottom = g.scene.TopChrome()
	g.mu.Lock()
	s.FooterTop = g.footerBar.Rect.Min.Y
	g.mu.Unlock()
	if r := g.referenceOpen(); r != nil && r.ref.Current().ID == ui.KeysEntry {
		g.mu.Lock()
		s.KeyListRect, s.KeyListScrolled, s.KeyListMax = r.keyList.Rect, r.keyList.Skip, r.keyList.Max
		g.mu.Unlock()
	}
	if j, ok := g.overlays.top().(*journal); ok {
		s.Journal = slices.Clone(j.entries[:j.drawn])
	}
	if f, ok := g.overlays.top().(*finder); ok && !g.hidden {
		f.mu.Lock()
		for _, h := range f.results {
			s.Found = append(s.Found, h.SessionID)
		}
		f.mu.Unlock()
	}
	g.shown = s
}

// driveSnapshot loads a home for the drive, with every session on the
// home's own scale of storeys, as the daemon puts them (item 109).
func driveSnapshot(home string, now time.Time) (state.Snapshot, error) {
	snapshot, err := state.NewLoader(home, state.Probes{Alive: state.ProcessAlive, Attached: state.UnixSocketConnected}).Load(now)
	if err != nil {
		return snapshot, err
	}
	state.NewHeights().Apply(snapshot.Sessions, true, now)
	return snapshot, nil
}
