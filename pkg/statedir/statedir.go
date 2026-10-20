// Package statedir is the one place Botropolis resolves its state
// directory. A run that is not the person's own city works in a private
// one, so it never reads or writes their layout, ledger or groups
// (item 111).
package statedir

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"

	"botropolis.city/botropolis/pkg/proc"
)

const appDir = "botropolis"

var (
	mu      sync.RWMutex
	private string
)

// started is the state directory this process started with: in a test
// binary, the developer's own.
var started = Default()

// Default is the person's own state directory. Resolving its name is
// always allowed, so a fence can compare against it; reading or writing
// under it goes through Dir or Check.
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

// Check fails a test that is about to read or write path when it is the
// state directory the run started with, or under it: the developer's
// own (item 165). Every writer under Default calls it.
func Check(path string) { guard(path, started, testing.Testing()) }

func guard(path, started string, inTest bool) {
	if !inTest {
		return
	}
	rel, err := filepath.Rel(started, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return
	}
	panic(fmt.Sprintf("statedir: a test reached the real state directory, %s; point XDG_STATE_HOME at t.TempDir() (item 165)", path))
}

// Dir is where every store keeps its file: the private directory when
// this process made one, the default otherwise.
func Dir() string {
	mu.RLock()
	defer mu.RUnlock()
	if private != "" {
		return private
	}
	dir := Default()
	Check(dir)
	return dir
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
	sweep()
	dir, err := os.MkdirTemp("", privatePattern)
	if err != nil {
		return func() error { return nil }, err
	}
	if err := os.WriteFile(filepath.Join(dir, OwnerFile), []byte(ownerLine(os.Getpid())), 0o600); err != nil {
		_ = os.RemoveAll(dir)
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

// Adopt works in a private directory another process made and will
// remove: a --headless city's parent makes it, so it is removed however
// the child ends (item 114). forget stops using it and leaves it there.
func Adopt(dir string) (forget func()) {
	mu.Lock()
	private = dir
	mu.Unlock()
	return func() {
		mu.Lock()
		if private == dir {
			private = ""
		}
		mu.Unlock()
	}
}

// OwnerFile names the process a private directory belongs to: its pid
// and, where the system says, when it started, so a pid handed to
// another process later does not keep the directory alive.
const OwnerFile = "owner"

const privatePattern = "botropolis-state-"

// sweep removes every private directory whose owner is gone: a run that
// died where no cleanup could run, in the GL driver, under a vanished X
// server or to SIGKILL (item 114). A directory with no owner file is
// left alone, since nothing says whose it is.
func sweep() {
	dirs, _ := filepath.Glob(filepath.Join(os.TempDir(), privatePattern+"*"))
	for _, dir := range dirs {
		data, err := os.ReadFile(filepath.Join(dir, OwnerFile))
		if err != nil {
			continue
		}
		var pid int
		var started string
		if n, _ := fmt.Sscan(string(data), &pid, &started); n < 1 || pid <= 0 {
			continue
		}
		if !alive(pid, started) {
			_ = os.RemoveAll(dir)
		}
	}
}

func ownerLine(pid int) string {
	return fmt.Sprintf("%d %s\n", pid, startTime(pid))
}

func alive(pid int, started string) bool {
	if err := syscall.Kill(pid, 0); err != nil && !errors.Is(err, syscall.EPERM) {
		return false
	}
	now := startTime(pid)
	return now == "-" || started == "" || started == "-" || now == started
}

// startTime is when pid started, as proc reads it; "-" where the
// system cannot say.
func startTime(pid int) string {
	started, ok := proc.StartTime(pid)
	if !ok {
		return "-"
	}
	return strconv.FormatUint(started, 10)
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
