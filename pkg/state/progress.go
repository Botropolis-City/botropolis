package state

// ScanProgress is how far a long background read of the history has
// got, in bytes, for the city to show while it runs (item 159).
type ScanProgress struct {
	Done  int64 `json:"done"`
	Known int64 `json:"known"`
}

// scanShown is the least a pass must have to read for its progress to
// be shown: every rescan starts a pass, and one that reads what a
// session just wrote is no wait worth a bar.
const scanShown = 32 << 20

func shown(done, known int64) *ScanProgress {
	if known < scanShown {
		return nil
	}
	return &ScanProgress{Done: min(done, known), Known: known}
}

// Progress is how far the running background read of the history has
// got, or nil when none worth showing is running.
func (l *Loader) Progress() *ScanProgress {
	if l.later != nil {
		if p := l.later.progress(); p != nil {
			return p
		}
	}
	switch {
	case l.stored != nil:
		return l.stored.progress()
	case l.whole != nil:
		return l.whole.progress()
	}
	return nil
}

// progressReporter is a statistics source that can say how far its
// ingest has got.
type progressReporter interface {
	Progress() (done, known int64)
}

func (h *storedHefts) progress() *ScanProgress {
	h.mu.Lock()
	running := h.running
	h.mu.Unlock()
	p, ok := h.src.(progressReporter)
	if !running || !ok {
		return nil
	}
	return shown(p.Progress())
}

func (w *wholeRead) progress() *ScanProgress {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.running {
		return nil
	}
	return shown(w.readBytes, w.knownBytes)
}
