// Package events is the city's event log: what happened to sessions, as
// seen by diffing one snapshot against the next. The daemon keeps the
// log and serves it over the socket; the map shows it.
package events

import (
	"fmt"
	"time"

	"botropolis.city/botropolis/pkg/claude"
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
	seen   map[string]seenSession
	// spare is the map the next Observe fills, the one before last kept
	// and cleared rather than made again (item 213).
	spare map[string]seenSession
}

// seenSession is what Observe compares of a session from one snapshot to
// the next. It keeps only that, since a copy of every session was most of
// what a rescan among 2,000 sessions allocated here (item 205).
type seenSession struct {
	state                  state.State
	apiErrors, compactions int
	prs                    []claude.PR
	title                  string
}

func seenOf(s *state.Session) seenSession {
	return seenSession{state: s.State, apiErrors: s.APIErrors, compactions: s.Compactions, prs: s.PRs, title: titleOf(s)}
}

func NewLog() *Log {
	return &Log{}
}

// Observe diffs a snapshot against the last and returns what is new.
func (l *Log) Observe(snapshot state.Snapshot, at time.Time) []Event {
	current := l.spare
	if current == nil {
		current = make(map[string]seenSession, len(snapshot.Sessions))
	}
	clear(current)
	for i := range snapshot.Sessions {
		current[snapshot.Sessions[i].ID] = seenOf(&snapshot.Sessions[i])
	}
	if l.seen == nil {
		l.seen = current
		return nil
	}
	var fresh []Event
	add := func(kind Kind, id, title, detail string) {
		fresh = append(fresh, Event{At: at, Kind: kind, SessionID: id, Title: title, Detail: detail})
	}
	for i := range snapshot.Sessions {
		s := &snapshot.Sessions[i]
		id := s.ID
		was, known := l.seen[id]
		if !known {
			if s.State != state.Resting {
				add(Started, id, titleOf(s), string(s.State))
			}
			continue
		}
		if s.State == state.NeedsYou && was.state != state.NeedsYou {
			add(NeedsYou, id, titleOf(s), s.Note)
		}
		if s.APIErrors > was.apiErrors {
			add(Error, id, titleOf(s), fmt.Sprintf("%d api errors", s.APIErrors))
		}
		if s.Compactions > was.compactions {
			add(Compaction, id, titleOf(s), fmt.Sprintf("compacted %dx", s.Compactions))
		}
		for _, pr := range s.PRs {
			old, had := findPR(was.prs, pr.Number)
			switch {
			case !had:
				add(PR, id, titleOf(s), fmt.Sprintf("#%d %s", pr.Number, pr.Repository))
			case pr.Merged() && !old.Merged():
				add(Merged, id, titleOf(s), fmt.Sprintf("#%d %s", pr.Number, pr.Repository))
			}
		}
	}
	for id, was := range l.seen {
		if _, still := current[id]; !still && was.state != state.Resting {
			add(Ended, id, was.title, string(was.state))
		}
	}
	l.seen, l.spare = current, l.seen
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

func findPR(prs []claude.PR, number int) (found interface{ Merged() bool }, ok bool) {
	for _, pr := range prs {
		if pr.Number == number {
			return pr, true
		}
	}
	return nil, false
}

func titleOf(s *state.Session) string {
	if s.Title != "" {
		return s.Title
	}
	return state.ShortID(s.ID)
}
