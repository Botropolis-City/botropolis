package daemon

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
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
// between into one, so the last of a burst is always read. The pace is
// at least a second, and long enough that the rescans take at most their
// share of a core, up to maxPace (item 205).
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
	files := d.addFileWatches(watcher)
	d.markWatching()

	var fire <-chan time.Time
	var last time.Time
	wait := pace
	schedule := func() {
		if fire == nil {
			fire = d.watchClock.After(max(debounce, last.Add(wait).Sub(d.watchClock.Now())))
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
			if files.watches(event) {
				appeared := files.appeared(event)
				if appeared {
					files = d.addFileWatches(watcher)
				}
				if appeared || files.files[event.Name] {
					schedule()
				}
				continue
			}
			if told != nil {
				told.Changed(event.Name)
			}
			schedule()
		case <-d.wake:
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
			fire, last = nil, d.watchClock.Now()
			cost, err := d.meteredRescan()
			wait = paceAfter(cost)
			if err != nil {
				continue
			}
			d.addWatches(watcher)
			files = d.addFileWatches(watcher)
		}
	}
}

// RequestRescan asks for a rescan on the watcher's pace, coalesced with
// any change it is waiting to read: a background pass that found
// something calls it, where calling Rescan at once let each pass start
// the next and the rescans outrun the pace (item 205). Until the watcher
// is up there is no pace to keep, so it rescans at once.
func (d *Daemon) RequestRescan() {
	select {
	case <-d.watching:
	default:
		_ = d.Rescan()
		return
	}
	select {
	case d.wake <- struct{}{}:
	default:
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

// FileWatcher is a loader that also reads files outside the directories
// it watches, such as the groups file the city writes a move to, which
// nothing else rescans for on a quiet home (item 365).
type FileWatcher interface {
	WatchFiles() []string
}

// fileWatches is the files the loader named and the directories watched
// for them: each file's own, or its nearest ancestor that exists, until
// it does. own are the directories the loader watches anyway, whose
// every event is the loader's.
type fileWatches struct {
	files map[string]bool
	dirs  map[string]bool
	own   map[string]bool
}

// addFileWatches watches the directory of every file the loader names.
// Only events on those files rescan: the state directory also holds the
// ledger and the statistics, which the daemon writes itself.
func (d *Daemon) addFileWatches(watcher *fsnotify.Watcher) fileWatches {
	fw, ok := d.loader.(FileWatcher)
	if !ok {
		return fileWatches{}
	}
	d.scanMu.Lock()
	files, own := fw.WatchFiles(), d.loader.WatchDirs()
	d.scanMu.Unlock()
	w := fileWatches{files: map[string]bool{}, dirs: map[string]bool{}, own: map[string]bool{}}
	for _, dir := range own {
		w.own[dir] = true
	}
	for _, file := range files {
		w.files[file] = true
		dir := existing(filepath.Dir(file))
		if w.dirs[dir] {
			continue
		}
		if err := d.addWatch(watcher, dir); err != nil {
			if !d.unwatchable[dir] {
				if d.unwatchable == nil {
					d.unwatchable = map[string]bool{}
				}
				d.unwatchable[dir] = true
				log.Printf("botropolisd: cannot watch %s (%v); a change to %s is read only on a rescan for something else", dir, err, file)
			}
			continue
		}
		w.dirs[dir] = true
	}
	return w
}

// existing is dir, or its nearest ancestor that exists.
func existing(dir string) string {
	for {
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		dir = parent
	}
}

// watches is whether event is in a directory watched only for files.
func (w fileWatches) watches(event fsnotify.Event) bool {
	dir := filepath.Dir(event.Name)
	return w.dirs[dir] && !w.own[dir]
}

// appeared is whether event made a directory on the way to a file. It is
// then watched in its ancestor's place, and read at once, since the file
// may be in it before the watch is.
func (w fileWatches) appeared(event fsnotify.Event) bool {
	if !event.Has(fsnotify.Create) {
		return false
	}
	for file := range w.files {
		rel, err := filepath.Rel(event.Name, filepath.Dir(file))
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func (d *Daemon) markWatching() {
	d.watchOnce.Do(func() { close(d.watching) })
}
