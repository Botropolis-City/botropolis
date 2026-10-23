package daemon

import "github.com/fsnotify/fsnotify"

// SetAddWatch replaces how the daemon adds a watch, so a test can have
// one fail.
func SetAddWatch(d *Daemon, add func(*fsnotify.Watcher, string) error) {
	d.addWatch = add
}
