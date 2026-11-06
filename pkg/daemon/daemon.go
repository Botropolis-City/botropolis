package daemon

import (
	"botropolis.city/botropolis/pkg/measure"
	"github.com/fsnotify/fsnotify"

	"context"
	"errors"
	"runtime/debug"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/find"
	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/state"
)

type Option func(*state.Loader)

// WithLedger keeps the ledger of sessions shown at path, and with it what
// is packed up (item 86).
func WithLedger(path string) Option {
	return func(l *state.Loader) { l.WithLedger(path) }
}

// WithCloseUp keeps the record of the last close-up at path (item 86).
func WithCloseUp(path string) Option {
	return func(l *state.Loader) { l.WithCloseUp(path) }
}

// CloseUpper is a loader that can record a close-up.
type CloseUpper interface {
	CloseUp(open, pinned []string, at time.Time) error
}

// CloseUp records that you closed up: which sessions were open and which
// you pinned to start with next time.
func (d *Daemon) CloseUp(open, pinned []string) error {
	c, ok := d.loader.(CloseUpper)
	if !ok {
		return state.ErrNoCloseUp
	}
	d.scanMu.Lock()
	defer d.scanMu.Unlock()
	return c.CloseUp(open, pinned, d.clock())
}

// Packer is a loader that can pack a session up or unpack it.
type Packer interface {
	Pack(id string, packed bool, at time.Time) error
}

// Pack packs a session up, or unpacks it, and rescans so every client
// sees it at once.
func (d *Daemon) Pack(id string, packed bool) error {
	p, ok := d.loader.(Packer)
	if !ok {
		return state.ErrNoLedger
	}
	d.scanMu.Lock()
	err := p.Pack(id, packed, d.clock())
	d.scanMu.Unlock()
	if err != nil {
		return err
	}
	return d.Rescan()
}

// Sweeper is a loader that can pack sessions up as the quiet sweep.
type Sweeper interface {
	Sweep(ids []string, at time.Time) error
}

// PackQuiet packs up, in one move, those of ids its own snapshot still
// holds quiet for at least quietFor, rescans, and says how many it
// packed (item 333).
func (d *Daemon) PackQuiet(ids []string, quietFor time.Duration) (int, error) {
	sw, ok := d.loader.(Sweeper)
	if !ok {
		return 0, state.ErrNoLedger
	}
	asked := make(map[string]bool, len(ids))
	for _, id := range ids {
		asked[id] = true
	}
	now := d.clock()
	var quiet []string
	for _, s := range d.Snapshot().Sessions {
		if asked[s.ID] && s.QuietFor(now, quietFor) {
			quiet = append(quiet, s.ID)
		}
	}
	d.scanMu.Lock()
	err := sw.Sweep(quiet, now)
	d.scanMu.Unlock()
	if err != nil || len(quiet) == 0 {
		return 0, err
	}
	return len(quiet), d.Rescan()
}

// Forgetter is a loader that can forget a demolished session.
type Forgetter interface {
	Forget(id string) error
}

// Forget drops a demolished session from the ledger and rescans, so
// every client stops showing it at once (item 144). A loader that keeps
// no ledger has nothing to forget.
func (d *Daemon) Forget(id string) error {
	f, ok := d.loader.(Forgetter)
	if !ok {
		return nil
	}
	d.scanMu.Lock()
	err := f.Forget(id)
	d.scanMu.Unlock()
	if err != nil {
		return err
	}
	return d.Rescan()
}

// Flusher is a loader holding state it has not saved yet (item 205).
type Flusher interface {
	Flush() error
}

// Flush has the loader save what it holds and has not saved: the daemon
// calls it when it stops.
func (d *Daemon) Flush() error {
	f, ok := d.loader.(Flusher)
	if !ok {
		return nil
	}
	d.scanMu.Lock()
	defer d.scanMu.Unlock()
	return f.Flush()
}

type overlay struct {
	at      time.Time
	state   state.State
	tool    string
	subject string
	note    string
	// turnEnded is an overlay from a hook that says only that the turn
	// ended, which a session still running its own shell command does
	// not need you for (item 250).
	turnEnded bool
}

type Snapshotter interface {
	Load(now time.Time) (state.Snapshot, error)
	WatchDirs() []string
}

type Daemon struct {
	loader Snapshotter
	// index is what was said in every session, for the find op; nil when
	// the daemon was made without a home. Item 82.
	index    *find.Index
	clock    func() time.Time
	scanMu   sync.Mutex
	watching chan struct{}
	// wake asks the watcher for a rescan on its own pace (item 205).
	wake      chan struct{}
	watchOnce sync.Once
	// unwatchable are the directories a watch could not be added for,
	// each logged once (item 205).
	unwatchable map[string]bool
	addWatch    func(*fsnotify.Watcher, string) error
	// meter is what a rescan cost a core, which paces the watcher's
	// rescans: its instructions where they can be counted, its thread's
	// CPU where not (items 205 and 213).
	meter func(func()) measure.Cost
	// watchClock is the time the watcher reads and waits on, the real
	// clock but in tests (item 213).
	watchClock watchClock
	// loaded, when not nil, is closed by the first scan to finish; until
	// then every request but a hook event waits for it (item 143).
	loaded   chan struct{}
	loadOnce sync.Once

	mu   sync.Mutex
	base state.Snapshot
	// store is what became of the statistics store (item 190).
	store    *state.StatsStore
	overlays map[string]overlay
	// shown is what each session last showed, and holds the sessions the
	// idle hold keeps showing as they were; shownAt counts presentations
	// so a session that is gone is forgotten (item 346).
	shown   map[string]shown
	holds   map[string]*hold
	shownAt uint64
	// hooked is every session a hook event has ever arrived for; a
	// session without one is only read from files, and the map says so.
	hooked      map[string]bool
	subscribers map[chan proto.Update]struct{}
	// log is what happened, as each view differs from the last; the
	// daemon keeps it so a client sees what it missed while closed.
	log *events.Log
	// release hands the heap's garbage back to the OS once hook events
	// and rescans have settled for settleAfter, or once ceiling has
	// passed since the first of them, so a daemon that loads and sees
	// no hook still returns its start-up peak, and one that never goes
	// quiet returns memory once a ceiling (items 157 and 208).
	release     func()
	settleAfter time.Duration
	ceiling     time.Duration
	unreleased  time.Time
	settle      Stopper
	// releaseClock is what the release timer reads and arms, the real
	// clock but in tests (item 205).
	releaseClock ReleaseClock
	// peer is what this daemon tells every client it is (item 149).
	peer atomic.Pointer[proto.Peer]
	// progressEvery and progressing pace and guard watchProgress.
	progressEvery time.Duration
	progressing   atomic.Bool
}

// IdleHold is how long a session that was working keeps showing so once
// its turn ends, so a pause of a second or two between turns does not
// flip it to needing you, log the event and send the notification. A
// permission prompt, a question, the session ending and the session
// working again all show at once. Item 346.
const IdleHold = 3 * time.Second

// SetPeer replaces the release and protocol the daemon says it is, for
// tests.
func (d *Daemon) SetPeer(p proto.Peer) { d.peer.Store(&p) }

// SettleAfter is how long after the last hook event or rescan the
// daemon returns memory to the OS.
const SettleAfter = 5 * time.Second

// ReleaseCeiling is the longest the daemon goes without returning memory
// while hook events or rescans keep it from settling.
const ReleaseCeiling = time.Minute

func New(home string, probes state.Probes, clock func() time.Time, options ...Option) *Daemon {
	loader := state.NewLoader(home, probes)
	for _, option := range options {
		option(loader)
	}
	d := NewWith(loader, clock)
	d.SetIndex(find.New(home))
	return d
}

// SetStatsStore tells the daemon what became of its statistics store,
// which it then says to every client (item 190).
func (d *Daemon) SetStatsStore(st state.StatsStore) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.store = &st
}

// statsStore is what SetStatsStore said, or nil.
func (d *Daemon) statsStore() *state.StatsStore {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.store
}

// SetIndex gives the daemon the index its find op answers from. A
// daemon built with NewWith has none until it is given one, and
// botropolisd is built that way. Item 82.
func (d *Daemon) SetIndex(ix *find.Index) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.index = ix
}

// Find refreshes the index from Claude Code's files and returns what a
// query finds, ranking the sessions that are running by their state.
func (d *Daemon) Find(q find.Query) ([]find.Hit, error) {
	d.mu.Lock()
	ix := d.index
	d.mu.Unlock()
	if ix == nil {
		return nil, errors.New("this daemon keeps no index")
	}
	if err := ix.Refresh(); err != nil {
		return nil, err
	}
	q.Live, q.Packed = map[string]state.State{}, map[string]bool{}
	for _, s := range d.Snapshot().Sessions {
		q.Live[s.ID] = s.State
		if s.Packed {
			q.Packed[s.ID] = true
		}
	}
	return ix.Find(q), nil
}

func NewWith(loader Snapshotter, clock func() time.Time) *Daemon {
	d := &Daemon{
		loader:      loader,
		clock:       clock,
		watching:    make(chan struct{}),
		wake:        make(chan struct{}, 1),
		overlays:    map[string]overlay{},
		shown:       map[string]shown{},
		holds:       map[string]*hold{},
		hooked:      map[string]bool{},
		subscribers: map[chan proto.Update]struct{}{},
		log:         events.NewLog(),
		release:     debug.FreeOSMemory,
		addWatch:    (*fsnotify.Watcher).Add,
		meter:       measure.Counted,
		watchClock:  realClock{},
		settleAfter: SettleAfter,
		ceiling:     ReleaseCeiling,

		releaseClock: realClock{},

		progressEvery: ProgressEvery,
	}
	d.SetPeer(proto.Self())
	return d
}

// SetRelease replaces how and how soon memory is returned after a burst
// of hook events or rescans, for tests and tuning.
func (d *Daemon) SetRelease(after time.Duration, release func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.settleAfter, d.release = after, release
}

// Stopper is a timer that can be stopped.
type Stopper interface{ Stop() bool }

// ReleaseClock is the time the release timer and the idle hold read, and
// how each arms.
type ReleaseClock interface {
	Now() time.Time
	AfterFunc(d time.Duration, f func()) Stopper
}

// SetReleaseClock replaces the clock the release timer and the idle hold
// read and arm, dropping what the old one had armed, for tests.
func (d *Daemon) SetReleaseClock(clock ReleaseClock) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.settle != nil {
		d.settle.Stop()
	}
	d.releaseClock, d.settle, d.unreleased = clock, nil, time.Time{}
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func (realClock) AfterFunc(d time.Duration, f func()) Stopper { return time.AfterFunc(d, f) }

func (realClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// watchClock is the time the watcher reads, and how it waits.
type watchClock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

// SetReleaseCeiling replaces the longest the daemon goes without
// returning memory while it never settles, for tests and tuning.
func (d *Daemon) SetReleaseCeiling(ceiling time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ceiling = ceiling
}

// scheduleRelease (re)arms the settle timer, but never past the ceiling
// counted from the first activity since the last release. Call it with
// mu held.
func (d *Daemon) scheduleRelease() {
	if d.settle != nil {
		d.settle.Stop()
	}
	now := d.releaseClock.Now()
	if d.unreleased.IsZero() {
		d.unreleased = now
	}
	after := min(d.settleAfter, d.unreleased.Add(d.ceiling).Sub(now))
	d.settle = d.releaseClock.AfterFunc(max(after, 0), d.settled)
}

func (d *Daemon) settled() {
	d.mu.Lock()
	d.unreleased = time.Time{}
	release := d.release
	d.mu.Unlock()
	release()
}

func (d *Daemon) Rescan() error {
	d.scanMu.Lock()
	snapshot, err := d.loader.Load(d.clock())
	d.scanMu.Unlock()
	if err != nil {
		return err
	}
	d.mu.Lock()
	// The loaded sessions are this rescan's own until they are the base,
	// so each says whether a hook has spoken for it here, and a view
	// with no hook overlay can share them (item 213).
	for i := range snapshot.Sessions {
		snapshot.Sessions[i].Hooked = d.hooked[snapshot.Sessions[i].ID]
	}
	d.base = snapshot
	d.pruneOverlays()
	view := d.present()
	d.scheduleRelease()
	d.mu.Unlock()
	d.notify(view)
	if d.loaded != nil {
		d.loadOnce.Do(func() { close(d.loaded) })
	}
	d.watchProgress()
	return nil
}

// HoldUntilLoaded has every request but a hook event wait for the first
// scan, so the daemon can serve before it has read the home and no
// client sees a city that is not there yet. Call it before Serve.
// Item 143.
func (d *Daemon) HoldUntilLoaded() {
	d.loaded = make(chan struct{})
}

func (d *Daemon) waitLoaded(ctx context.Context) bool {
	if d.loaded == nil {
		return true
	}
	select {
	case <-d.loaded:
		return true
	case <-ctx.Done():
		return false
	}
}

func (d *Daemon) Snapshot() state.Snapshot {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.view()
}

func (d *Daemon) Apply(event claude.HookEvent, at time.Time) {
	d.mu.Lock()
	if event.SessionID != "" {
		d.hooked[event.SessionID] = true
	}
	o, known := d.overlays[event.SessionID]
	if !known {
		o = overlay{}
	}
	o.at = at
	o.turnEnded = event.Name == claude.HookStop ||
		(event.Name == claude.HookNotification && event.NotificationType == claude.NotificationIdlePrompt)
	switch event.Name {
	case claude.HookPreToolUse:
		o.state, o.tool, o.subject, o.note = state.Working, event.ToolName, event.Subject(), ""
	case claude.HookPostToolUse, claude.HookUserPromptSubmit:
		o.state, o.tool, o.subject, o.note = state.Working, "", "", ""
	case claude.HookNotification:
		o.state, o.tool, o.subject, o.note = state.NeedsYou, "", "", event.Message
	case claude.HookStop:
		o.state, o.tool, o.subject, o.note = state.NeedsYou, "", "", ""
	case claude.HookSessionEnd:
		o.state, o.tool, o.subject, o.note = state.Resting, "", "", ""
	case claude.HookSessionStart:
		delete(d.overlays, event.SessionID)
		view := d.present()
		d.mu.Unlock()
		d.notify(view)
		return
	default:
		if !known {
			d.mu.Unlock()
			return
		}
	}
	d.overlays[event.SessionID] = o
	view := d.present()
	d.scheduleRelease()
	d.mu.Unlock()
	d.notify(view)
}

// Subscribe delivers every change from now on, with the events each one
// logged.
func (d *Daemon) Subscribe() (<-chan proto.Update, func()) {
	_, updates, cancel := d.Attach(time.Time{}, false)
	return updates, cancel
}

// Attach is a subscription that also hands over the current view and
// the log since a moment (the whole log when zero), taken in the same
// breath so nothing is missed or delivered twice; withBacklog false
// skips the log.
func (d *Daemon) Attach(since time.Time, withBacklog bool) (proto.Update, <-chan proto.Update, func()) {
	ch := make(chan proto.Update, 1)
	d.mu.Lock()
	d.subscribers[ch] = struct{}{}
	first := proto.Update{Snapshot: d.view()}
	if withBacklog {
		first.Events = d.log.Since(since)
	}
	d.mu.Unlock()
	return first, ch, func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		if _, ok := d.subscribers[ch]; ok {
			delete(d.subscribers, ch)
			close(ch)
		}
	}
}

// Events is the log after a moment (all of it when zero), newest first.
func (d *Daemon) Events(since time.Time) []events.Event {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.log.Since(since)
}

// shown is what a session showed: what the idle hold keeps.
type shown struct {
	state               state.State
	tool, subject, note string
	at                  uint64
}

// hold is a session the idle hold keeps showing as it was, and the timer
// that ends the hold (item 346).
type hold struct {
	shown shown
	timer Stopper
}

// view is what every client reads: the base the files gave, the hook
// overlays that speak for sessions, and the sessions the idle hold still
// shows as they were. Call it with mu held.
func (d *Daemon) view() state.Snapshot {
	view, copied := d.raw()
	d.applyHolds(&view, copied)
	return view
}

// present is view where state has changed: it decides, from what the
// view would show against what each session last showed, which sessions
// the idle hold keeps as they were, so a session going from working to
// idle shows working until IdleHold passes with nothing new, and
// anything else shows at once (item 346). Call it with mu held, from
// wherever the base, an overlay or a hold changes; Snapshot only reads.
func (d *Daemon) present() state.Snapshot {
	view, copied := d.raw()
	d.shownAt++
	for i := range view.Sessions {
		s := &view.Sessions[i]
		h, holding := d.holds[s.ID]
		idle := s.State == state.NeedsYou && !d.prompted(i)
		switch {
		case holding && !idle:
			h.timer.Stop()
			delete(d.holds, s.ID)
			holding = false
		case !holding && idle:
			was, known := d.shown[s.ID]
			if known && (was.state == state.Working || was.state == state.Unattended) {
				id := s.ID
				h = &hold{shown: was}
				h.timer = d.releaseClock.AfterFunc(IdleHold, func() { d.holdPassed(id, h) })
				d.holds[id] = h
				holding = true
			}
		}
		now := shown{state: s.State, tool: s.Tool, subject: s.Subject, note: s.Note, at: d.shownAt}
		if holding {
			now = h.shown
			now.at = d.shownAt
		}
		d.shown[s.ID] = now
	}
	for id, was := range d.shown {
		if was.at != d.shownAt {
			delete(d.shown, id)
			if h, ok := d.holds[id]; ok {
				h.timer.Stop()
				delete(d.holds, id)
			}
		}
	}
	d.applyHolds(&view, copied)
	return view
}

// prompted is whether the session at i in the base needs you for
// something a hold must not delay: a permission prompt or a question,
// rather than a turn that ended.
func (d *Daemon) prompted(i int) bool {
	base := &d.base.Sessions[i]
	if base.RecordStatus == claude.StatusWaiting || base.Turn == claude.TurnNeedsInput {
		return true
	}
	o, applies := d.applying(base)
	return applies && o.state == state.NeedsYou && !o.turnEnded
}

// holdPassed ends a session's hold once IdleHold has passed with nothing
// new, and shows what it has been holding back.
func (d *Daemon) holdPassed(id string, h *hold) {
	d.mu.Lock()
	if d.holds[id] != h {
		d.mu.Unlock()
		return
	}
	delete(d.holds, id)
	d.shown[id] = shown{state: state.NeedsYou, at: d.shownAt}
	view := d.present()
	d.mu.Unlock()
	d.notify(view)
}

// applyHolds shows every held session as it was, copying the sessions
// first if the raw view still shares them with the base.
func (d *Daemon) applyHolds(view *state.Snapshot, copied bool) {
	if len(d.holds) == 0 {
		return
	}
	for i := range view.Sessions {
		h, ok := d.holds[view.Sessions[i].ID]
		if !ok {
			continue
		}
		if !copied {
			view.Sessions, copied = slices.Clone(view.Sessions), true
		}
		s := &view.Sessions[i]
		s.State, s.Tool, s.Subject, s.Note = h.shown.state, h.shown.tool, h.shown.subject, h.shown.note
	}
}

// applying is the hook overlay that speaks for a session, if one does.
func (d *Daemon) applying(base *state.Session) (overlay, bool) {
	o, ok := d.overlays[base.ID]
	return o, ok && o.at.After(base.LastActivity) && (!o.turnEnded || base.RecordStatus != claude.StatusShell) &&
		(o.state != state.Working || base.RecordStatus != claude.StatusWaiting)
}

// raw is the base with the hook overlays applied and no hold, and
// whether its sessions were copied from the base's or still share them.
func (d *Daemon) raw() (state.Snapshot, bool) {
	view := d.base
	view.Store = d.store
	copied := false
	for i := range d.base.Sessions {
		base := &d.base.Sessions[i]
		hooked := d.hooked[base.ID]
		o, applies := d.applying(base)
		if hooked == base.Hooked && !applies {
			continue
		}
		if !copied {
			view.Sessions, copied = slices.Clone(d.base.Sessions), true
		}
		s := &view.Sessions[i]
		s.Hooked = hooked
		if !applies {
			continue
		}
		s.LastActivity = o.at
		s.Tool = o.tool
		s.Subject = o.subject
		s.Note = o.note
		switch o.state {
		case state.Working:
			if s.Kind == claude.KindBackground && !s.Attached {
				s.State = state.Unattended
			} else {
				s.State = state.Working
			}
		case state.NeedsYou:
			// A session waiting on its own watch handed the turn back on
			// purpose; the Stop hook does not make it need you.
			if s.State != state.Waiting {
				s.State = o.state
			}
		case state.Resting:
			s.State = o.state
		}
	}
	return view, copied
}

func (d *Daemon) pruneOverlays() {
	for _, s := range d.base.Sessions {
		if o, ok := d.overlays[s.ID]; ok && !o.at.After(s.LastActivity) {
			delete(d.overlays, s.ID)
		}
	}
}

// notify logs what changed and hands every subscriber the new view with
// those events; a subscriber that has not taken the last update gets
// the newer view with both batches, so no event is lost.
func (d *Daemon) notify(snapshot state.Snapshot) {
	d.mu.Lock()
	defer d.mu.Unlock()
	fresh := d.log.Observe(snapshot, d.clock())
	for ch := range d.subscribers {
		update := proto.Update{Snapshot: snapshot, Events: fresh}
		select {
		case ch <- update:
		default:
			select {
			case stale := <-ch:
				update.Events = append(append([]events.Event(nil), fresh...), stale.Events...)
			default:
			}
			ch <- update
		}
	}
}
