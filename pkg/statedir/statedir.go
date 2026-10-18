// Package statedir is the one place Botropolis resolves its state
// directory. A run that is not the person's own city works in a private
// one, so it never reads or writes their layout, ledger or groups
// (item 111).
package statedir

import (
	"os"
	"path/filepath"
	"sync"
)

const appDir = "botropolis"

var (
	mu      sync.RWMutex
	private string
)

// Default is the person's own state directory.
func Default() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(".", appDir)
		}
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, appDir)
}

// Dir is where every store keeps its file: the private directory when
// this process made one, the default otherwise.
func Dir() string {
	mu.RLock()
	defer mu.RUnlock()
	if private != "" {
		return private
	}
	return Default()
}

// IsPrivate reports whether this process works in a private directory.
func IsPrivate() bool {
	mu.RLock()
	defer mu.RUnlock()
	return private != ""
}

// MakePrivate gives this process a temporary state directory holding a
// copy of seed's files, if seed exists, and returns what removes it.
// Copying means a seed in a checkout is never written to.
func MakePrivate(seed string) (func() error, error) {
	dir, err := os.MkdirTemp("", "botropolis-state-")
	if err != nil {
		return func() error { return nil }, err
	}
	cleanup := func() error {
		mu.Lock()
		if private == dir {
			private = ""
		}
		mu.Unlock()
		return os.RemoveAll(dir)
	}
	if err := copyFiles(seed, dir); err != nil {
		_ = cleanup()
		return func() error { return nil }, err
	}
	mu.Lock()
	private = dir
	mu.Unlock()
	return cleanup, nil
}

func copyFiles(src, dst string) error {
	entries, err := os.ReadDir(src)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), data, 0o600); err != nil {
			return err
		}
	}
	return nil
}
