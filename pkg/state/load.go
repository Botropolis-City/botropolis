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
	HeftsRead bool      `json:"heftsRead,omitempty"`
	At        time.Time `json:"at"`
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
