package stats

import (
	"errors"
	"sync"
	"time"
)

const (
	// LedgerRetry is how long a ledger waits before trying a store that
	// would not open again.
	LedgerRetry = 5 * time.Second
	// LedgerCache is how long a window's answer is reused whatever is
	// written. A window over the real history costs about 300 ms to
	// read, and read every 2 s it was 3.6% of a core (item 215).
	LedgerCache = 30 * time.Second
	// LedgerAging is how long a window is reused while nothing is
	// written, since messages still leave it as the clock moves.
	LedgerAging = 5 * time.Minute
)

// ReasonOff is a nil ledger's reason: statistics are turned off.
const ReasonOff = "statistics are off"

// Ledger is the statistics store as a reader in another process sees
// it: opened read-only, tried again while it will not open, and with
// each window's answer reused for a moment, since the panels that show
// them draw every frame. A nil Ledger is statistics turned off. Every
// call takes the moment it is made, so a recorded or scripted run reads
// its own clock. Item 116.
type Ledger struct {
	path  string
	mu    sync.Mutex
	store *Store
	err   error
	tried time.Time
	// live is each window as last read, by its hours, and costs every
	// session's cost, each brought up to date from the store's change
	// log (item 215).
	live  map[int]*liveWindow
	costs *liveCosts
	// due are the windows, by their hours, and costsDue the costs, that
	// ReadFresh has the next ask read whatever is cached (item 242).
	due      map[int]bool
	costsDue bool
	// priced and messageReads count the sessions priced and the reads
	// of messages, for the tests.
	priced, messageReads int
}

// fresh is whether an answer read at from, when the store's data
// version was version, still stands at at: always within cache, and
// while nothing was written within ageing, if it is not zero.
func fresh(s *Store, from, at time.Time, version int64, cache, ageing time.Duration) bool {
	if at.Before(from) {
		return false
	}
	age := at.Sub(from)
	if age < cache {
		return true
	}
	if ageing > 0 && age >= ageing {
		return false
	}
	now, err := s.DataVersion()
	return err == nil && now == version
}

// NewLedger is a ledger over the store at path, opened when first asked.
func NewLedger(path string) *Ledger {
	return &Ledger{path: path, live: map[int]*liveWindow{}, due: map[int]bool{}}
}

// Store is the open store, opening it if it is due a try.
func (l *Ledger) Store(at time.Time) (*Store, bool) {
	if l == nil {
		return nil, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.storeLocked(at)
}

func (l *Ledger) storeLocked(at time.Time) (*Store, bool) {
	if l.store != nil {
		return l.store, true
	}
	if !l.tried.IsZero() && at.Sub(l.tried) < LedgerRetry {
		return nil, false
	}
	l.tried = at
	l.store, l.err = OpenReader(l.path)
	return l.store, l.store != nil
}

// Reason is why the ledger cannot be read, in plain words, or "" when
// it can.
func (l *Ledger) Reason() string {
	if l == nil {
		return ReasonOff
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.store != nil {
		return ""
	}
	if l.err == nil {
		return Reason(ErrNoStore)
	}
	return Reason(l.err)
}

// Window is the last hours up to at, reused for LedgerCache, and for
// LedgerAging while nothing is written.
func (l *Ledger) Window(at time.Time, hours int, root func(string) string) (Window, error) {
	if l == nil {
		return Window{}, errors.New(ReasonOff)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	s, ok := l.storeLocked(at)
	if !ok {
		if l.err == nil {
			return Window{}, ErrNoStore
		}
		return Window{}, l.err
	}
	if w, ok := l.live[hours]; ok && !l.due[hours] && fresh(s, w.at, at, w.version, LedgerCache, LedgerAging) {
		return w.window, nil
	}
	delete(l.due, hours)
	return l.windowLocked(at, hours, root)
}

// windowLocked reads the window now, from what changed since it was
// last read.
func (l *Ledger) windowLocked(at time.Time, hours int, root func(string) string) (Window, error) {
	s, ok := l.storeLocked(at)
	if !ok {
		if l.err == nil {
			return Window{}, ErrNoStore
		}
		return Window{}, l.err
	}
	version, err := s.DataVersion()
	if err != nil {
		return Window{}, err
	}
	w, err := s.refresh(l.live[hours], at, hours, &l.messageReads)
	if err != nil {
		delete(l.live, hours)
		return Window{}, err
	}
	answer, err := w.answer(at, hours, root)
	if err != nil {
		delete(l.live, hours)
		return Window{}, err
	}
	w.at, w.version, w.window = at, version, answer
	l.live[hours] = w
	return answer, nil
}

// ReadFresh has the next ask for each window, and for the session
// costs, read the store rather than reuse a cached answer; the reuse
// holds again from that read on. The city asks it when the history's
// first read finishes, so it never shows a window cached partway
// through (item 242).
func (l *Ledger) ReadFresh() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for hours := range l.live {
		l.due[hours] = true
	}
	l.costsDue = l.costs != nil
}

// Close closes the store, if it was opened.
func (l *Ledger) Close() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.store != nil {
		_ = l.store.Close()
		l.store = nil
	}
}
