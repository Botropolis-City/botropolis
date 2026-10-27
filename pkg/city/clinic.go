package city

import (
	"fmt"
	"slices"
	"time"

	"botropolis.city/botropolis/pkg/format"
	"botropolis.city/botropolis/pkg/state"
)

// The clinic stands for what needs you now: the sessions waiting on you,
// then the ones in trouble, and clicking it lists them (item 280; Aria,
// 2026-10-26).

const (
	ClinicWidth  = 72.0
	ClinicHeight = 48.0
)

type Clinic struct {
	Rect Rect
}

// ClinicSection is which of the clinic's two lists a row is in.
type ClinicSection string

const (
	ClinicNeedsYou  ClinicSection = "needs you"
	ClinicInTrouble ClinicSection = "in trouble"
)

// ClinicRow is one session the clinic lists, and why.
type ClinicRow struct {
	Building *Building
	Section  ClinicSection
	Reason   string
}

// Clinic is every session the clinic lists: those needing you, in the
// triage walk's order, then those in trouble, the most recent first.
func (s *Scene) Clinic() []ClinicRow {
	now := s.now()
	var rows []ClinicRow
	for _, b := range s.TriageWalk() {
		reason := b.Session.Doing()
		if reason == "" {
			reason = "waiting " + format.Age(now.Sub(b.Session.LastActivity))
		}
		rows = append(rows, ClinicRow{Building: b, Section: ClinicNeedsYou, Reason: reason})
	}
	type troubled struct {
		b      *Building
		reason string
		at     time.Time
	}
	var trouble []troubled
	for _, b := range s.city.inTroubleMaybe() {
		if reason, at, ok := troubleOf(b, now); ok {
			trouble = append(trouble, troubled{b, reason, at})
		}
	}
	slices.SortFunc(trouble, func(a, b troubled) int {
		if c := b.at.Compare(a.at); c != 0 {
			return c
		}
		return state.LongestWaiting(a.b.Session, b.b.Session)
	})
	for _, t := range trouble {
		rows = append(rows, ClinicRow{Building: t.b, Section: ClinicInTrouble, Reason: t.reason})
	}
	return rows
}

// ClinicCount is how many sessions the clinic lists. The standing key
// and the plate ask for it every frame, so it counts over the city's
// lists of what needs you and what may be in trouble, and builds no
// rows.
func (s *Scene) ClinicCount() int {
	now := s.now()
	n := 0
	for _, b := range s.city.waitingOnYou() {
		s.clinicLooks++
		if s.inTriage(b) {
			n++
		}
	}
	for _, b := range s.city.inTroubleMaybe() {
		s.clinicLooks++
		if _, _, ok := troubleOf(b, now); ok {
			n++
		}
	}
	return n
}

// troubleCandidate is a building that troubleOf may find in trouble,
// whatever the time: all of its tests but how long ago it stopped.
func troubleCandidate(b *Building) bool {
	sess := b.Session
	if b.Packed || sess.Packed || sess.State == state.NeedsYou {
		return false
	}
	live := state.Live(sess.State)
	return (live && sess.APIErrors > 0 && !sess.LastErrorAt.Before(sess.LastActivity)) ||
		(!live && sess.StoppedMidTurn) ||
		(live && sess.FailedTools >= FailedToolsInARow)
}

// StoppedFor is how long a session that stopped mid-turn stays in the
// clinic's list (Aria, 2026-10-26).
const StoppedFor = 24 * time.Hour

// FailedToolsInARow is how many tool calls in a row must fail before a
// session is in trouble: one failing is routine, as a red test is.
const FailedToolsInARow = 3

// troubleOf is why a session not needing you is in trouble, and since
// when, the first that applies: its last record is an error from the
// API, which a reply that succeeds clears; it stopped mid-turn within
// StoppedFor, a crash or a terminal closed; or its last tool calls
// failed, FailedToolsInARow or more of them (item 280).
func troubleOf(b *Building, now time.Time) (string, time.Time, bool) {
	if !troubleCandidate(b) {
		return "", time.Time{}, false
	}
	sess := b.Session
	live := state.Live(sess.State)
	switch {
	case live && sess.APIErrors > 0 && !sess.LastErrorAt.Before(sess.LastActivity):
		return "API error " + format.Age(now.Sub(sess.LastErrorAt)) + " ago", sess.LastErrorAt, true
	case !live && sess.StoppedMidTurn && now.Sub(sess.LastActivity) <= StoppedFor:
		return "stopped mid-turn " + format.Age(now.Sub(sess.LastActivity)) + " ago", sess.LastActivity, true
	case live && sess.FailedTools >= FailedToolsInARow:
		return fmt.Sprintf("%d tool calls failed in a row", sess.FailedTools), sess.LastActivity, true
	}
	return "", time.Time{}, false
}

func (s *Scene) clinicCard() Card {
	var needs, trouble int
	for _, r := range s.Clinic() {
		if r.Section == ClinicNeedsYou {
			needs++
		} else {
			trouble++
		}
	}
	line := "nobody needs you and nothing is in trouble"
	if needs+trouble > 0 {
		line = fmt.Sprintf("%s · %d in trouble", CountLabel(state.Count{State: state.NeedsYou, N: needs}), trouble)
	}
	return Card{Title: "Clinic", Lines: []string{line, "click or press a for the list"}}
}
