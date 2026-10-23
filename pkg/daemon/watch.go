package daemon

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	debounce = 100 * time.Millisecond
	// pace is the least time between two rescans the watcher starts,
	// so a home whose sessions write many times a second costs a rescan
	// a second however many are writing; hooks still change a building
	// at once (item 196).
	pace = time.Second
)

// ChangeWatcher is a loader the watcher tells what changed, so a rescan
// need not look at every file to find out (item 205).
type ChangeWatcher interface {
	Changed(path string)
	Watching(dir string, ok bool)
	Unwatched()
}

// Watch rescans after a change: within the debounce after a quiet spell,
// and otherwise a pace after the last rescan, coalescing every change in
// between into one, so the last of a burst is always read.
func (d *Daemon) Watch(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer func() { _ = watcher.Close() }()
	told, _ := d.loader.(ChangeWatcher)
	if told != nil {
		defer told.Unwatched()
	}
	d.addWatches(watcher)
	d.markWatching()

	var fire <-chan time.Time
	var last time.Time
	schedule := func() {
		if fire == nil {
			fire = time.After(max(debounce, time.Until(last.Add(pace))))
		}
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if told != nil {
				told.Changed(event.Name)
			}
			schedule()
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			if told != nil && errors.Is(err, fsnotify.ErrEventOverflow) {
				told.Unwatched()
				schedule()
			}
		case <-fire:
			fire, last = nil, time.Now()
			if err := d.Rescan(); err != nil {
				continue
			}
			d.addWatches(watcher)
		}
	}
}

func (d *Daemon) WaitWatching(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-d.watching:
		return nil
	}
}

// addWatches watches every directory the loader reads, and tells it which
// it could not: those it looks at in full on every rescan, and each is
// logged the first time, since a directory nothing watches is one whose
// changes only a rescan for something else would find.
func (d *Daemon) addWatches(watcher *fsnotify.Watcher) {
	d.scanMu.Lock()
	dirs := d.loader.WatchDirs()
	d.scanMu.Unlock()
	told, _ := d.loader.(ChangeWatcher)
	for _, dir := range dirs {
		err := d.addWatch(watcher, dir)
		if told != nil {
			told.Watching(dir, err == nil)
		}
		if err != nil && !d.unwatchable[dir] {
			if d.unwatchable == nil {
				d.unwatchable = map[string]bool{}
			}
			d.unwatchable[dir] = true
			log.Printf("botropolisd: cannot watch %s (%v); its transcripts are looked at on every rescan instead", dir, err)
		}
	}
}

func (d *Daemon) markWatching() {
	d.watchOnce.Do(func() { close(d.watching) })
}
