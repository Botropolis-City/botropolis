package state

import (
	"maps"
	"runtime"
	"sync"
	"time"

	"botropolis.city/botropolis/pkg/claude"
)

// StatsSource is the statistics store as the loader needs it: brought up
// to date from the transcripts, and every stored session's counters.
type StatsSource interface {
	Ingest(projectsDir string, now time.Time) error
	Hefts() (map[string]claude.Usage, error)
}

// storedHefts takes hefts from the statistics store instead of reading
// stopped transcripts whole: what was stored is read at start-up, and the
// store is brought up to date in the background, one pass at a time.
// Sessions whose transcripts are gone keep their hefts. Item 110.
type storedHefts struct {
	src     StatsSource
	done    func()
	mu      sync.Mutex
	hefts   map[string]claude.Usage
	read    bool
	running bool
}

// WithStats takes every session's heft from src, and calls done after a
// background refresh changes them so the caller can load again. The
// hefts src already holds are read now, so a restart starts on the
// user's own scale.
func (l *Loader) WithStats(src StatsSource, done func()) *Loader {
	hefts, err := src.Hefts()
	if err != nil {
		hefts = nil
	}
	l.stored = &storedHefts{src: src, done: done, hefts: hefts, read: len(hefts) > 0}
	return l
}

// apply sets every session's heft, from the store for a session read
// through a window, and starts a refresh. It reports whether the hefts
// are the user's whole history, and gives every stored heft for the
// scale.
func (h *storedHefts) apply(sessions []Session, projectsDir string, now time.Time) (bool, []int64) {
	h.mu.Lock()
	hefts, read := h.hefts, h.read
	h.mu.Unlock()
	for i := range sessions {
		s := &sessions[i]
		s.Heft, s.HeftPartial = Heft(s.Usage), s.Usage.Partial
		if !s.Usage.Partial {
			continue
		}
		if u, ok := hefts[s.ID]; ok {
			s.Heft, s.HeftPartial = Heft(u), false
		}
	}
	var sample []int64
	for _, u := range hefts {
		if heft := Heft(u); heft > 0 {
			sample = append(sample, heft)
		}
	}
	h.start(projectsDir, now)
	return read, sample
}

func (h *storedHefts) start(projectsDir string, now time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.running {
		return
	}
	h.running = true
	go h.pass(projectsDir, now)
}

func (h *storedHefts) pass(projectsDir string, now time.Time) {
	runtime.LockOSThread()
	lowerPriority()
	changed := false
	if err := h.src.Ingest(projectsDir, now); err == nil {
		if hefts, err := h.src.Hefts(); err == nil {
			h.mu.Lock()
			changed = !h.read || !maps.Equal(hefts, h.hefts)
			h.hefts, h.read = hefts, true
			h.mu.Unlock()
		}
	}
	h.mu.Lock()
	h.running = false
	h.mu.Unlock()
	if changed && h.done != nil {
		h.done()
	}
}

// ApplyOn is Apply with the scale cut from sample, which may hold hefts
// of sessions no longer shown.
func (h *Heights) ApplyOn(sessions []Session, sample []int64, read bool, now time.Time) {
	h.apply(sessions, sample, read, now)
}

// SampleOf is the hefts of the sessions a scale may be cut from: whole,
// and of a conversation that is still there.
func SampleOf(sessions []Session) []int64 {
	var sample []int64
	for _, s := range sessions {
		if s.Heft > 0 && !s.HeftPartial && !s.ConversationGone {
			sample = append(sample, s.Heft)
		}
	}
	return sample
}
