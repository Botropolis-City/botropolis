package stats

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// offMarker is the file that says statistics were turned off with the
// data kept. While they are off the database is not opened, so the
// choice is kept beside it instead.
const offMarker = "stats-off"

// prunedNotes lists the sessions prune removed while statistics were
// off, for turning them back on to forget.
const prunedNotes = "stats-pruned"

// Prompt is the question the city owes about the statistics setting.
type Prompt int

const (
	NoPrompt Prompt = iota
	// OffPrompt asks whether to keep or delete what is stored, because
	// statistics are off and nobody has said what to do with it.
	OffPrompt
	// OnPrompt asks whether to start fresh or keep the old data with a
	// gap, because statistics are on again over kept data.
	OnPrompt
)

// Owed is the prompt the city owes when it opens with the setting on or
// off, from the files in the state directory dir. A setting changed where
// no prompt could be shown is caught here. Item 110.
func Owed(dir string, on bool) Prompt {
	marked := exists(filepath.Join(dir, offMarker))
	switch {
	case on && marked:
		return OnPrompt
	case !on && !marked && exists(filepath.Join(dir, file)):
		return OffPrompt
	}
	return NoPrompt
}

// TurnOff answers the off prompt: keep leaves the database and marks it
// kept, and otherwise the database is deleted.
func TurnOff(dir string, keep bool) error {
	if keep {
		return os.WriteFile(filepath.Join(dir, offMarker), nil, 0o600)
	}
	return removeAll(dir)
}

// TurnOn answers the on prompt: keepOld records the stretch from when the
// database was last written to now as a gap, and otherwise the old data
// is deleted so the store starts fresh.
func TurnOn(dir string, keepOld bool, now time.Time) error {
	path := filepath.Join(dir, file)
	if !keepOld || !exists(path) {
		return removeAll(dir)
	}
	off, err := lastWritten(path)
	if err != nil {
		return err
	}
	pruned, err := noted(dir)
	if err != nil {
		return err
	}
	s, err := Open(path)
	if err != nil {
		return err
	}
	if err := s.Forget(pruned...); err != nil {
		_ = s.Close()
		return err
	}
	if _, err := s.db.Exec("INSERT INTO gaps (off_at, on_at) VALUES (?, ?)", off.UnixMilli(), now.UnixMilli()); err != nil {
		_ = s.Close()
		return err
	}
	if err := s.Close(); err != nil {
		return err
	}
	if err := removeIfThere(filepath.Join(dir, prunedNotes)); err != nil {
		return err
	}
	return removeIfThere(filepath.Join(dir, offMarker))
}

// NotePruned records, while statistics are off over a database, a session
// prune removed, so turning them back on with the data kept forgets it.
// That holds whether the data was kept at the prompt or the prompt is
// still owed. With no database there is nothing to forget, and nothing is
// written. The database itself is not opened: its last write is where
// the gap starts.
func NotePruned(dir, session string) error {
	if !exists(filepath.Join(dir, file)) {
		return nil
	}
	f, err := os.OpenFile(filepath.Join(dir, prunedNotes), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(session + "\n"); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// OffSince is when the kept database was last written, which is where a
// gap starts, and whether there is one.
func OffSince(dir string) (time.Time, bool) {
	at, err := lastWritten(filepath.Join(dir, file))
	return at, err == nil
}

func noted(dir string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(dir, prunedNotes))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var sessions []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			sessions = append(sessions, line)
		}
	}
	return sessions, nil
}

func lastWritten(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	latest := info.ModTime()
	if wal, err := os.Stat(path + "-wal"); err == nil && wal.ModTime().After(latest) {
		latest = wal.ModTime()
	}
	return latest, nil
}

func removeAll(dir string) error {
	for _, name := range []string{file, file + "-wal", file + "-shm", offMarker, prunedNotes} {
		if err := removeIfThere(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}

func removeIfThere(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
