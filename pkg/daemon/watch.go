package daemon

import (
	"context"
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

// Watch rescans after a change: within the debounce after a quiet spell,
// and otherwise a pace after the last rescan, coalescing every change in
// between into one, so the last of a burst is always read.
func (d *Daemon) Watch(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer func() { _ = watcher.Close() }()
	d.addWatches(watcher)
	d.markWatching()

	var fire <-chan time.Time
	var last time.Time
	for {
		select {
		case <-ctx.Done():
			return nil
		case _, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if fire == nil {
				fire = time.After(max(debounce, time.Until(last.Add(pace))))
			}
		case _, ok := <-watcher.Errors:
			if !ok {
				return nil
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

func (d *Daemon) addWatches(watcher *fsnotify.Watcher) {
	d.scanMu.Lock()
	dirs := d.loader.WatchDirs()
	d.scanMu.Unlock()
	for _, dir := range dirs {
		_ = watcher.Add(dir)
	}
}

func (d *Daemon) markWatching() {
	d.watchOnce.Do(func() { close(d.watching) })
}
