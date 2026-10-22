package state

import (
	"runtime"
	"sync"
	"sync/atomic"

	"botropolis.city/botropolis/pkg/claude"
)

// wholeRead reads stopped transcripts whole, one at a time and in the
// background, for their heft: the catalogue's window holds only a head
// and a tail. A stopped transcript never changes, so each is read once
// and held in memory by path, size and modification time (item 109).
type wholeRead struct {
	done     func()
	pace     *pace
	readFile func(path string) (int64, bool)
	mu       sync.Mutex
	hefts    map[string]cachedHeft
	lane     chan struct{}
	running  bool
	passed   bool
	reads    atomic.Int64
	// readBytes and knownBytes are the running pass's bytes read and to read.
	readBytes, knownBytes int64
}

type cachedHeft struct {
	key  fileKey
	heft int64
	ok   bool
}

type stoppedFile struct {
	path string
	key  fileKey
}

// WithWholeRead reads every stopped transcript whole in the background
// after a load, for its heft, and calls done when a pass has finished
// so the caller can load again. Without it, stopped sessions' hefts stay
// lower bounds and the scale stays on the starter lines.
func (l *Loader) WithWholeRead(done func()) *Loader {
	l.whole = &wholeRead{done: done, pace: newPace(), readFile: readHeft, hefts: map[string]cachedHeft{}, lane: l.lane}
	l.later = newLaterRead(done, l.gate.yield, l.lane)
	return l
}

// WholeReads is how many transcripts have been read whole.
func (l *Loader) WholeReads() int {
	if l.whole == nil {
		return 0
	}
	return int(l.whole.reads.Load())
}

// applyHefts sets every session's heft, from its usage when that is the
// whole session's and from the whole read when not, and starts a pass
// over whatever has not been read. It reports whether every session's
// heft has been read whole at least once.
func (l *Loader) applyHefts(sessions []Session, stopped map[string]stoppedFile) bool {
	var pending []stoppedFile
	for i := range sessions {
		s := &sessions[i]
		s.Heft, s.HeftPartial = Heft(s.Usage), s.Usage.Partial
		if !s.Usage.Partial {
			continue
		}
		f, ok := stopped[s.ID]
		if !ok || l.whole == nil {
			continue
		}
		if heft, read := l.whole.heft(f); read {
			s.Heft, s.HeftPartial = heft, false
			continue
		}
		pending = append(pending, f)
	}
	if l.whole == nil {
		return false
	}
	return l.whole.start(pending)
}

func (w *wholeRead) heft(f stoppedFile) (int64, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	c, ok := w.hefts[f.path]
	return c.heft, ok && c.ok && c.key == f.key
}

// start begins a pass over pending unless one is running, and reports
// whether a pass has ever finished.
func (w *wholeRead) start(pending []stoppedFile) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	var todo []stoppedFile
	for _, f := range pending {
		if c, ok := w.hefts[f.path]; !ok || c.key != f.key {
			todo = append(todo, f)
		}
	}
	if w.running {
		return w.passed
	}
	if len(todo) == 0 {
		w.passed = true
		return true
	}
	w.running = true
	go w.pass(todo)
	return w.passed
}

func (w *wholeRead) pass(todo []stoppedFile) {
	runtime.LockOSThread()
	backgroundPriority()
	takeLane(w.lane)
	w.pace.start()
	w.mu.Lock()
	w.readBytes, w.knownBytes = 0, 0
	for _, f := range todo {
		w.knownBytes += f.key.size
	}
	w.mu.Unlock()
	for _, f := range todo {
		heft, ok := w.readFile(f.path)
		w.reads.Add(1)
		w.mu.Lock()
		w.hefts[f.path] = cachedHeft{key: f.key, heft: heft, ok: ok}
		w.readBytes += f.key.size
		w.mu.Unlock()
		w.pace.yield(func() {})
	}
	w.mu.Lock()
	w.running, w.passed = false, true
	w.mu.Unlock()
	freeLane(w.lane)
	if w.done != nil {
		w.done()
	}
}

func readHeft(path string) (int64, bool) {
	t, err := claude.ReadTranscript(path)
	if err != nil {
		return 0, false
	}
	usage := t.Usage
	subs, _, err := claude.ReadSubagents(path)
	if err != nil {
		return 0, false
	}
	for _, sub := range subs {
		usage = usage.Add(sub.Usage)
	}
	return Heft(usage), true
}
