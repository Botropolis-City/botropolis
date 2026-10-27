package city

import (
	"fmt"
	"slices"

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
	var trouble []*Building
	for _, b := range s.city.Buildings() {
		if inTrouble(b) {
			trouble = append(trouble, b)
		}
	}
	slices.SortFunc(trouble, func(a, b *Building) int {
		if c := b.Session.LastErrorAt.Compare(a.Session.LastErrorAt); c != 0 {
			return c
		}
		return state.LongestWaiting(a.Session, b.Session)
	})
	for _, b := range trouble {
		rows = append(rows, ClinicRow{Building: b, Section: ClinicInTrouble,
			Reason: "API error " + format.Age(now.Sub(b.Session.LastErrorAt)) + " ago"})
	}
	return rows
}

// ClinicCount is how many sessions the clinic lists.
func (s *Scene) ClinicCount() int {
	return len(s.Clinic())
}

// inTrouble is a live session that does not need you whose last record
// is an error from the API: a reply that succeeds after it clears it.
func inTrouble(b *Building) bool {
	sess := b.Session
	if b.Packed || sess.Packed || !state.Live(sess.State) || sess.State == state.NeedsYou {
		return false
	}
	return sess.APIErrors > 0 && !sess.LastErrorAt.Before(sess.LastActivity)
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
