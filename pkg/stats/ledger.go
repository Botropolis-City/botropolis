package stats

import (
	"sync"
	"time"
)

const (
	// LedgerRetry is how long a ledger waits before trying a store that
	// would not open again.
	LedgerRetry = 5 * time.Second
	// LedgerCache is how long a window's answer is reused.
	LedgerCache = 2 * time.Second
)

// reasonOff is a nil ledger's reason: statistics are turned off.
const reasonOff = "statistics are off"

type cachedWindow struct {
	at     time.Time
	window Window
}

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
	cache map[int]cachedWindow
}

// NewLedger is a ledger over the store at path, opened when first asked.
func NewLedger(path string) *Ledger {
	return &Ledger{path: path, cache: map[int]cachedWindow{}}
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
		return reasonOff
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

// Window is the last hours up to at, reused for LedgerCache.
func (l *Ledger) Window(at time.Time, hours int, root func(string) string) (Window, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if c, ok := l.cache[hours]; ok && at.Sub(c.at) < LedgerCache && !at.Before(c.at) {
		return c.window, nil
	}
	s, ok := l.storeLocked(at)
	if !ok {
		if l.err == nil {
			return Window{}, ErrNoStore
		}
		return Window{}, l.err
	}
	w, err := s.Window(at.Add(-time.Duration(hours)*time.Hour), at, hours, root)
	if err != nil {
		return w, err
	}
	l.cache[hours] = cachedWindow{at: at, window: w}
	return w, nil
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
