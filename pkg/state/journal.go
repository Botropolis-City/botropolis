package state

import (
	"strings"
	"time"
)

const notRecorded = "not recorded"

// Journal is a group's entry on the board: what happened last time,
// what is next, and when it was last active, all read from what Claude
// already wrote (item 87).
type Journal struct {
	LastTime string    `json:"lastTime"`
	Next     string    `json:"next"`
	Active   time.Time `json:"active"`
}

// GroupJournal reads the entry from the most recently active member
// that has a recap, split at the recap's own "Next:", or failing any
// recap from the most recently active member's last prompt.
func GroupJournal(members []Session) Journal {
	var latest, recapped *Session
	for i := range members {
		m := &members[i]
		if latest == nil || m.LastActivity.After(latest.LastActivity) {
			latest = m
		}
		if strings.TrimSpace(m.Recap.Text) != "" && (recapped == nil || m.LastActivity.After(recapped.LastActivity)) {
			recapped = m
		}
	}
	if latest == nil {
		return Journal{Next: notRecorded}
	}
	j := Journal{Active: latest.LastActivity, Next: notRecorded}
	if recapped == nil {
		j.LastTime = strings.TrimSpace(latest.LastPrompt)
		return j
	}
	recap := strings.TrimSpace(recapped.Recap.Text)
	j.LastTime = recap
	if before, after, ok := strings.Cut(recap, "Next:"); ok {
		j.LastTime = strings.TrimSpace(before)
		if next := strings.TrimSpace(after); next != "" {
			j.Next = next
		}
	}
	return j
}
