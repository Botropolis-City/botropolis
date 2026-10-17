package daemon

import (
	"context"
	"time"

	"github.com/fsnotify/fsnotify"
)

const debounce = 100 * time.Millisecond

func (d *Daemon) Watch(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer func() { _ = watcher.Close() }()
	d.addWatches(watcher)
	d.markWatching()

	var timer *time.Timer
	var fire <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			return nil
		case _, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if timer == nil {
				timer = time.NewTimer(debounce)
				fire = timer.C
			} else {
				timer.Reset(debounce)
			}
		case _, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
		case <-fire:
			timer, fire = nil, nil
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
