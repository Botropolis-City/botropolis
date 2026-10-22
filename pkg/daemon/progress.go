package daemon

import (
	"time"

	"botropolis.city/botropolis/pkg/state"
)

// ProgressEvery is how often the daemon looks at a long read of the
// history and sends what has moved (item 159).
const ProgressEvery = time.Second

// progressQuiet is how many looks without a scan a watch makes before
// it stops: a rescan's pass may not yet have counted what it has to
// read when the rescan returns.
const progressQuiet = 3

type progressor interface {
	Progress() *state.ScanProgress
}

// LoadProgress is how far the loader's load has got, or nothing of
// nothing when it cannot say.
func (d *Daemon) LoadProgress() state.LoadProgress {
	if p, ok := d.loader.(interface{ LoadProgress() state.LoadProgress }); ok {
		return p.LoadProgress()
	}
	return state.LoadProgress{}
}

// SetProgressEvery replaces how often progress is sent, for tests.
func (d *Daemon) SetProgressEvery(every time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.progressEvery = every
}

// watchProgress, after a rescan, sends subscribers the scan's progress
// as it moves, without loading again, until no scan has been running
// for a few looks. One runs at a time.
func (d *Daemon) watchProgress() {
	p, ok := d.loader.(progressor)
	if !ok || !d.progressing.CompareAndSwap(false, true) {
		return
	}
	d.mu.Lock()
	every, last := d.progressEvery, d.base.Scan
	d.mu.Unlock()
	go func() {
		defer d.progressing.Store(false)
		for quiet := 0; quiet < progressQuiet; {
			time.Sleep(every)
			scan := p.Progress()
			if scan == nil {
				quiet++
			} else {
				quiet = 0
			}
			if same(scan, last) {
				continue
			}
			last = scan
			d.mu.Lock()
			d.base.Scan = scan
			view := d.view()
			d.mu.Unlock()
			d.notify(view)
		}
	}()
}

func same(a, b *state.ScanProgress) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
