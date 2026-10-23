package daemon

import (
	"github.com/fsnotify/fsnotify"

	"context"
	"errors"
	"runtime/debug"
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
}

type Snapshotter interface {
	Load(now time.Time) (state.Snapshot, error)
	WatchDirs() []string
}

type Daemon struct {
	loader Snapshotter
	// index is what was said in every session, for the find op; nil when
	// the daemon was made without a home. Item 82.
	index     *find.Index
	clock     func() time.Time
	scanMu    sync.Mutex
	watching  chan struct{}
	watchOnce sync.Once
	// unwatchable are the directories a watch could not be added for,
	// each logged once (item 205).
	unwatchable map[string]bool
	addWatch    func(*fsnotify.Watcher, string) error
	// loaded, when not nil, is closed by the first scan to finish; until
	// then every request but a hook event waits for it (item 143).
	loaded   chan struct{}
	loadOnce sync.Once

	mu       sync.Mutex
	base     state.Snapshot
	overlays map[string]overlay
	// hooked is every session a hook event has ever arrived for; a
	// session without one is only read from files, and the map says so.
	hooked      map[string]bool
	subscribers map[chan proto.Update]struct{}
	// log is what happened, as each view differs from the last; the
	// daemon keeps it so a client sees what it missed while closed.
	log *events.Log
	// release hands the heap's garbage back to the OS once a burst of
	// hook events has settled for settleAfter; only rescans did before,
	// so a storm left the daemon sitting at its ceiling.
	release     func()
	settleAfter time.Duration
	settle      *time.Timer
	// peer is what this daemon tells every client it is (item 149).
	peer atomic.Pointer[proto.Peer]
	// progressEvery and progressing pace and guard watchProgress.
	progressEvery time.Duration
	progressing   atomic.Bool
}

// SetPeer replaces the release and protocol the daemon says it is, for
// tests.
func (d *Daemon) SetPeer(p proto.Peer) { d.peer.Store(&p) }

// SettleAfter is how long after the last hook event the daemon returns
// memory to the OS.
const SettleAfter = 5 * time.Second

func New(home string, probes state.Probes, clock func() time.Time, options ...Option) *Daemon {
	loader := state.NewLoader(home, probes)
	for _, option := range options {
		option(loader)
	}
	d := NewWith(loader, clock)
	d.SetIndex(find.New(home))
	return d
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
		overlays:    map[string]overlay{},
		hooked:      map[string]bool{},
		subscribers: map[chan proto.Update]struct{}{},
		log:         events.NewLog(),
		release:     debug.FreeOSMemory,
		addWatch:    (*fsnotify.Watcher).Add,
		settleAfter: SettleAfter,

		progressEvery: ProgressEvery,
	}
	d.SetPeer(proto.Self())
	return d
}

// SetRelease replaces how and how soon memory is returned after a burst
// of hook events, for tests and tuning.
func (d *Daemon) SetRelease(after time.Duration, release func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.settleAfter, d.release = after, release
}

// scheduleRelease (re)arms the settle timer; the burst is over when it
// fires.
func (d *Daemon) scheduleRelease() {
	if d.settle != nil {
		d.settle.Stop()
	}
	d.settle = time.AfterFunc(d.settleAfter, d.release)
}

func (d *Daemon) Rescan() error {
	d.scanMu.Lock()
	snapshot, err := d.loader.Load(d.clock())
	d.scanMu.Unlock()
	if err != nil {
		return err
	}
	d.mu.Lock()
	d.base = snapshot
	d.pruneOverlays()
	view := d.view()
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
		view := d.view()
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
	view := d.view()
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

func (d *Daemon) view() state.Snapshot {
	view := d.base
	view.Sessions = make([]state.Session, len(d.base.Sessions))
	copy(view.Sessions, d.base.Sessions)
	for i := range view.Sessions {
		s := &view.Sessions[i]
		s.Hooked = d.hooked[s.ID]
		o, ok := d.overlays[s.ID]
		if !ok || !o.at.After(s.LastActivity) {
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
	return view
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
