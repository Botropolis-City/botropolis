package city

import (
	"fmt"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/format"
	"botropolis.city/botropolis/pkg/state"
)

// Generated is a summary botropolis asked Claude Code for, as distinct
// from the recap Claude Code wrote into the transcript itself: while it
// runs, once it is back, or why it failed.
type Generated struct {
	Text    string
	At      time.Time
	Pending bool
	Err     string
}

// SetGenerated records where a generated summary for a session stands.
func (s *Scene) SetGenerated(id string, g Generated) {
	if s.generated == nil {
		s.generated = map[string]Generated{}
	}
	s.generated[id] = g
}

// ToggleDetails folds the selection's details out or away from the
// keyboard, which is the card's button by another route.
func (s *Scene) ToggleDetails() (Action, string) {
	if s.selected == nil {
		return Action{}, "select a building first"
	}
	if s.detailsOpen() {
		return s.Act(ActionDetailsHide)
	}
	return s.Act(ActionDetails)
}

// summarisable is whether a session's transcript is Claude Code's, the
// only kind that carries a recap or that the summariser can read.
func summarisable(session state.Session) bool {
	return session.Harness == "" || session.Harness == "claude"
}

// DetailsOpen is whether the selection's card shows its details (i).
func (s *Scene) DetailsOpen() bool { return s.detailsOpen() }

func (s *Scene) detailsOpen() bool {
	return s.selected != nil && s.detailsFor == s.selected.Session.ID
}

// summaryActions is the summary's part of the selection's actions: read,
// the level below the card's sentence (item 83), and Generate, on a
// session with no recap of its own and nothing already running.
func (s *Scene) summaryActions() []ActionKind {
	session := s.selected.Session
	if !summarisable(session) {
		return nil
	}
	kinds := []ActionKind{ActionRead}
	if session.Recap.Text == "" && !s.generated[session.ID].Pending {
		kinds = append(kinds, ActionGenerate)
	}
	return kinds
}

// buildingCard is a building's card as the scene shows it: read through
// the current view, leading with a generated summary where that is the
// best sentence there is, and saying in its details where the sentence
// came from.
func (s *Scene) buildingCard(b *Building) Card {
	g := s.generated[b.Session.ID]
	card := b.face(s.city.Time, s.view, g.Text)
	if summarisable(b.Session) {
		card.Details = append([]string{s.summary(b.Session)}, card.Details...)
	}
	// The tool the main thread is using, which the rover carried until
	// the building showed its own work (item 89).
	if b.Session.Tool != "" {
		card.Details = append(card.Details, "now      "+b.Session.Tool)
	}
	return card
}

// summary is the details line that says what the summary is: Claude
// Code's own recap with its age, or a generated one, each labelled as
// what it is.
func (s *Scene) summary(session state.Session) string {
	if r := session.Recap; r.Text != "" {
		return "recap    " + format.Age(s.city.Time.Sub(r.At)) + " old, " + since(r.PromptsSince)
	}
	g := s.generated[session.ID]
	switch {
	case g.Pending:
		return "summary  generating with " + claude.SummaryModel + "…"
	case g.Err != "":
		return "summary  failed: " + g.Err
	case g.Text != "":
		return "summary  generated " + format.Age(s.city.Time.Sub(g.At)) + " ago with " + claude.SummaryModel
	}
	return "recap    none yet"
}

func since(prompts int) string {
	switch prompts {
	case 0:
		return "nothing since"
	case 1:
		return "1 turn since"
	}
	return fmt.Sprintf("%d turns since", prompts)
}
