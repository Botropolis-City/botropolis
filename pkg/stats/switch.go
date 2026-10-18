package stats

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

// offMarker is the file that says statistics were turned off with the
// data kept. While they are off the database is not opened, so the
// choice is kept beside it instead.
const offMarker = "stats-off"

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
	s, err := Open(path)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec("INSERT INTO gaps (off_at, on_at) VALUES (?, ?)", off.UnixMilli(), now.UnixMilli()); err != nil {
		_ = s.Close()
		return err
	}
	if err := s.Close(); err != nil {
		return err
	}
	return removeIfThere(filepath.Join(dir, offMarker))
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
	for _, name := range []string{file, file + "-wal", file + "-shm", offMarker} {
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
