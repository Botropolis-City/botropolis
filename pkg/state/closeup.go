package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const closeUpFile = "closeup.json"

// CloseUp is the last time you closed up: when, which sessions were open,
// and which you pinned to start with next time (item 86, slice 4). It
// holds ids and a time, and never anything Claude or you wrote.
type CloseUp struct {
	At     time.Time `json:"at"`
	Open   []string  `json:"open,omitempty"`
	Pinned []string  `json:"pinned,omitempty"`
}

// CloseUpPath is where the daemon keeps the last close-up, beside the
// ledger.
func CloseUpPath() string {
	return filepath.Join(filepath.Dir(LedgerPath()), closeUpFile)
}

func LoadCloseUp(path string) (CloseUp, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return CloseUp{}, nil
	}
	if err != nil {
		return CloseUp{}, err
	}
	var c CloseUp
	return c, json.Unmarshal(data, &c)
}

func (c CloseUp) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ErrNoCloseUp is a close-up asked of a loader that keeps no record of
// one.
var ErrNoCloseUp = errors.New("this daemon keeps no close-up record, so it cannot close up")

// WithCloseUp keeps the last close-up at path.
func (l *Loader) WithCloseUp(path string) *Loader {
	l.closeUpPath = path
	return l
}

// CloseUp records a close-up, replacing the last one.
func (l *Loader) CloseUp(open, pinned []string, at time.Time) error {
	if l.closeUpPath == "" {
		return ErrNoCloseUp
	}
	return CloseUp{At: at, Open: open, Pinned: pinned}.Save(l.closeUpPath)
}

// LastCloseUp is the last close-up recorded, or none.
func (l *Loader) LastCloseUp() (CloseUp, error) {
	if l.closeUpPath == "" {
		return CloseUp{}, nil
	}
	return LoadCloseUp(l.closeUpPath)
}
