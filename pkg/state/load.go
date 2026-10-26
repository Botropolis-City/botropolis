package state

import (
	"errors"
	"syscall"
	"time"

	"botropolis.city/botropolis/pkg/claude"
)

type Snapshot struct {
	Sessions []Session     `json:"sessions"`
	Servers  []Server      `json:"servers"`
	Skills   []Skill       `json:"skills"`
	Roads    []Road        `json:"roads"`
	Teams    []claude.Team `json:"teams,omitempty"`
	Groups   []Group       `json:"groups,omitempty"`
	// GroupsReviewed is whether the first run's proposal of groups has
	// been confirmed (item 87).
	GroupsReviewed bool                 `json:"groupsReviewed,omitempty"`
	Power          Power                `json:"power"`
	Stats          *claude.Stats        `json:"stats,omitempty"`
	Skipped        []claude.SkippedFile `json:"skipped"`
	// HeftsRead is whether every session's heft has been read whole at
	// least once, so the user's own scale can be cut from them (item 109).
	HeftsRead bool `json:"heftsRead,omitempty"`
	// ScaleHefts is every stored session's heft, sessions whose
	// transcripts are gone included, for the scale to be cut from when
	// statistics are kept. It never leaves the daemon (item 110).
	ScaleHefts []int64 `json:"-"`
	// Scan is how far a long background read of the history has got,
	// while one runs (item 159).
	Scan *ScanProgress `json:"scan,omitempty"`
	// Store is what became of the daemon's statistics store, on every
	// snapshot a daemon sends (item 190).
	Store *StatsStore `json:"store,omitempty"`
	At    time.Time   `json:"at"`
}

func Load(home string, probes Probes, now time.Time) (Snapshot, error) {
	return NewLoader(home, probes).Load(now)
}

func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
