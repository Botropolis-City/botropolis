// Package events is the city's event log: what happened to sessions, as
// seen by diffing one snapshot against the next. The daemon keeps the
// log and serves it over the socket; the map shows it.
package events

import (
	"fmt"
	"time"

	"botropolis.city/botropolis/pkg/state"
)

// Kind is what happened to a session.
type Kind string

const (
	NeedsYou   Kind = "needs-you"
	Error      Kind = "error"
	PR         Kind = "pr"
	Merged     Kind = "merged"
	Compaction Kind = "compaction"
	Started    Kind = "started"
	Ended      Kind = "ended"
)

// Event is one thing that happened, as the daemon saw it happen.
type Event struct {
	At        time.Time `json:"at"`
	Kind      Kind      `json:"kind"`
	SessionID string    `json:"session_id"`
	Title     string    `json:"title"`
	Detail    string    `json:"detail,omitempty"`
}

// Keep is how many events the log holds.
const Keep = 500

// Log turns the stream of snapshots into events by what changed between
// one and the next; the first snapshot only sets the baseline.
type Log struct {
	events []Event
	seen   map[string]state.Session
}

func NewLog() *Log {
	return &Log{}
}

// Observe diffs a snapshot against the last and returns what is new.
func (l *Log) Observe(snapshot state.Snapshot, at time.Time) []Event {
	current := map[string]state.Session{}
	for _, s := range snapshot.Sessions {
		current[s.ID] = s
	}
	if l.seen == nil {
		l.seen = current
		return nil
	}
	var fresh []Event
	add := func(kind Kind, s state.Session, detail string) {
		fresh = append(fresh, Event{At: at, Kind: kind, SessionID: s.ID, Title: titleOf(s), Detail: detail})
	}
	for id, s := range current {
		was, known := l.seen[id]
		if !known {
			if s.State != state.Parked {
				add(Started, s, string(s.State))
			}
			continue
		}
		if s.State == state.NeedsYou && was.State != state.NeedsYou {
			add(NeedsYou, s, s.Note)
		}
		if s.APIErrors > was.APIErrors {
			add(Error, s, fmt.Sprintf("%d api errors", s.APIErrors))
		}
		if s.Compactions > was.Compactions {
			add(Compaction, s, fmt.Sprintf("compacted %dx", s.Compactions))
		}
		for _, pr := range s.PRs {
			old, had := findPR(was, pr.Number)
			switch {
			case !had:
				add(PR, s, fmt.Sprintf("#%d %s", pr.Number, pr.Repository))
			case pr.Merged() && !old.Merged():
				add(Merged, s, fmt.Sprintf("#%d %s", pr.Number, pr.Repository))
			}
		}
	}
	for id, was := range l.seen {
		if _, still := current[id]; !still && was.State != state.Parked {
			add(Ended, was, string(was.State))
		}
	}
	l.seen = current
	l.Add(fresh...)
	return fresh
}

// Add puts events at the head of the log, newest first; a batch keeps
// its own order, and the oldest fall off the end.
func (l *Log) Add(fresh ...Event) {
	if len(fresh) == 0 {
		return
	}
	l.events = append(append([]Event(nil), fresh...), l.events...)
	if len(l.events) > Keep {
		l.events = l.events[:Keep]
	}
}

// Events is the log, newest first.
func (l *Log) Events() []Event {
	return l.events
}

// Since is the events after a moment (every event when the moment is
// zero), of the given kinds or of every kind when none is given, newest
// first.
func (l *Log) Since(t time.Time, kinds ...Kind) []Event {
	var out []Event
	for _, e := range l.events {
		if !e.At.After(t) {
			continue
		}
		if len(kinds) > 0 && !hasKind(kinds, e.Kind) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func hasKind(kinds []Kind, k Kind) bool {
	for _, kind := range kinds {
		if kind == k {
			return true
		}
	}
	return false
}

func findPR(s state.Session, number int) (found interface{ Merged() bool }, ok bool) {
	for _, pr := range s.PRs {
		if pr.Number == number {
			return pr, true
		}
	}
	return nil, false
}

func titleOf(s state.Session) string {
	if s.Title != "" {
		return s.Title
	}
	return state.ShortID(s.ID)
}
