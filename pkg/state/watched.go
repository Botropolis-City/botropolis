package state

import (
	"path/filepath"
	"sync"
)

// auditEvery is how many loads a stopped transcript in a watched
// directory can go without being looked at: each load looks at one in
// this many, so a change the watcher missed is read within as many loads
// (item 205).
const auditEvery = 60

// watchState is what the daemon's watcher has said since the last load:
// which directories it holds, and which files in them changed. A loader
// nobody tells anything trusts no directory, and looks at every stopped
// transcript on every load (item 205).
type watchState struct {
	mu      sync.Mutex
	held    map[string]bool
	fresh   map[string]bool
	changed map[string]bool
	all     bool
}

// watchView is one load's share of it.
type watchView struct {
	held, fresh, changed map[string]bool
	all                  bool
}

// Changed is the watcher saying a file changed.
func (l *Loader) Changed(path string) {
	w := &l.watch
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.changed == nil {
		w.changed = map[string]bool{}
	}
	w.changed[path] = true
}

// Unwatched is the watcher saying it may have missed changes, as when
// its queue overflowed: the next load looks at every transcript.
func (l *Loader) Unwatched() {
	w := &l.watch
	w.mu.Lock()
	defer w.mu.Unlock()
	w.all = true
}

// Watching is whether the watcher holds a directory. One it has just
// started holding is looked at once more, for what changed before the
// watch; one it could not hold is looked at on every load.
func (l *Loader) Watching(dir string, ok bool) {
	w := &l.watch
	w.mu.Lock()
	defer w.mu.Unlock()
	if !ok {
		delete(w.held, dir)
		return
	}
	if w.held[dir] {
		return
	}
	if w.held == nil {
		w.held, w.fresh = map[string]bool{}, map[string]bool{}
	}
	w.held[dir], w.fresh[dir] = true, true
}

// take is this load's view, leaving the changes for the next to gather.
func (w *watchState) take() watchView {
	w.mu.Lock()
	defer w.mu.Unlock()
	held := make(map[string]bool, len(w.held))
	for dir := range w.held {
		held[dir] = true
	}
	v := watchView{held: held, fresh: w.fresh, changed: w.changed, all: w.all}
	w.fresh, w.changed, w.all = map[string]bool{}, nil, false
	return v
}

// trusts is whether the watcher would have said if path had changed.
func (v watchView) trusts(path string) bool {
	dir := filepath.Dir(path)
	return !v.all && v.held[dir] && !v.fresh[dir] && !v.changed[path]
}

// TranscriptStats is how many times the loader has looked at a stopped
// transcript's size and modification time.
func (l *Loader) TranscriptStats() int {
	return l.transcriptStats
}
