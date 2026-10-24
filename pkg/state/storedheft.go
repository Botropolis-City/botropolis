package state

import (
	"maps"
	"runtime"
	"sort"
	"sync"
	"time"

	"botropolis.city/botropolis/pkg/claude"
)

// StatsSource is the statistics store as the loader needs it: brought up
// to date from the transcripts, every stored session's counters, and
// the counters of only the sessions that moved since they were last
// read, a moved session without counters being one forgotten (item 196).
// Ingest calls yield often, and yield waits while a load runs.
type StatsSource interface {
	Ingest(projectsDir string, now time.Time, yield func()) error
	Hefts() (map[string]claude.Usage, error)
	MovedHefts() (map[string]claude.Usage, []string, error)
}

// PathIngester is a statistics store that can bring only the
// transcripts named up to date, for a loader that knows which changed
// (item 205).
type PathIngester interface {
	IngestPaths(projectsDir string, paths []string, listed bool, now time.Time, yield func()) error
}

// fullIngestEvery is how often a pass ingests every transcript all the
// same, for what the loader never looks at, such as the subagents of a
// session whose main transcript is gone (item 205).
const fullIngestEvery = 10 * time.Minute

// storedHefts takes hefts from the statistics store instead of reading
// stopped transcripts whole: what was stored is read at start-up, and the
// store is brought up to date in the background, one pass at a time.
// Sessions whose transcripts are gone keep their hefts. Item 110.
type storedHefts struct {
	src     StatsSource
	yield   func()
	pace    *pace
	done    func()
	lane    chan struct{}
	mu      sync.Mutex
	hefts   map[string]claude.Usage
	read    bool
	running bool
	// pending are the transcripts that changed since the last pass, and
	// listed whether any appeared or went; fullAt is when the last pass
	// that ingested everything began (item 205).
	pending map[string]bool
	listed  bool
	fullAt  time.Time
	clock   func() time.Time
}

// WithStats takes every session's heft from src, and calls done after a
// background refresh changes them so the caller can load again. The
// hefts src already holds are read now, so a restart starts on the
// user's own scale.
func (l *Loader) WithStats(src StatsSource, done func()) *Loader {
	_, _, _ = src.MovedHefts()
	hefts, err := src.Hefts()
	if err != nil {
		hefts = nil
	}
	l.stored = &storedHefts{src: src, yield: l.gate.yield, pace: newPace(), done: done, hefts: hefts, read: len(hefts) > 0, lane: l.lane, pending: map[string]bool{}, clock: time.Now}
	l.later = newLaterRead(done, l.gate.yield, l.lane)
	return l
}

// WithFullSpeed has the background passes read for the whole clock
// rather than half a core: a capture waits on the history's read and
// nothing else does (item 240). It follows WithStats or WithWholeRead.
func (l *Loader) WithFullSpeed() *Loader {
	if l.stored != nil {
		l.stored.pace.fraction = 1
	}
	if l.whole != nil {
		l.whole.pace.fraction = 1
	}
	if l.later != nil {
		l.later.pace.fraction = 1
	}
	return l
}

// apply sets every session's heft, from the store for a session read
// through a window, and starts a refresh. It reports whether the hefts
// are the user's whole history, and gives every stored heft for the
// scale.
func (h *storedHefts) apply(sessions []Session, projectsDir string, now time.Time, touched []string, listed bool) (bool, []int64) {
	h.mu.Lock()
	hefts, read := h.hefts, h.read
	for _, path := range touched {
		h.pending[path] = true
	}
	h.listed = h.listed || listed
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
	_, paths := h.src.(PathIngester)
	full := !paths || h.fullAt.IsZero() || h.clock().Sub(h.fullAt) >= fullIngestEvery
	if !full && len(h.pending) == 0 && !h.listed {
		return
	}
	var changed []string
	for path := range h.pending {
		changed = append(changed, path)
	}
	sort.Strings(changed)
	listed := h.listed
	h.pending, h.listed = map[string]bool{}, false
	if full {
		h.fullAt = h.clock()
	}
	h.running = true
	go h.pass(projectsDir, now, full, changed, listed)
}

// ingest brings the store up to date: every transcript, or only those
// that changed.
func (h *storedHefts) ingest(projectsDir string, now time.Time, full bool, changed []string, listed bool, yield func()) error {
	if full {
		return h.src.Ingest(projectsDir, now, yield)
	}
	return h.src.(PathIngester).IngestPaths(projectsDir, changed, listed, now, yield)
}

func (h *storedHefts) pass(projectsDir string, now time.Time, full bool, touched []string, listed bool) {
	runtime.LockOSThread()
	backgroundPriority()
	takeLane(h.lane)
	h.pace.start()
	yield := func() { h.pace.yield(h.yield) }
	changed := false
	err := h.ingest(projectsDir, now, full, touched, listed, yield)
	if err != nil {
		h.mu.Lock()
		h.fullAt = time.Time{}
		h.mu.Unlock()
	}
	if err == nil {
		if moved, ids, err := h.src.MovedHefts(); err == nil {
			h.mu.Lock()
			changed = !h.read
			hefts := h.hefts
			if len(ids) > 0 {
				hefts = maps.Clone(h.hefts)
			}
			if hefts == nil {
				hefts = map[string]claude.Usage{}
			}
			for _, id := range ids {
				u, ok := moved[id]
				if old, had := hefts[id]; ok == had && u == old {
					continue
				}
				changed = true
				if ok {
					hefts[id] = u
				} else {
					delete(hefts, id)
				}
			}
			h.hefts, h.read = hefts, true
			h.mu.Unlock()
		}
	}
	h.mu.Lock()
	h.running = false
	h.mu.Unlock()
	freeLane(h.lane)
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
