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

// ToggleSummary opens or closes the selection's summary from the
// keyboard, which is the card's button by another route.
func (s *Scene) ToggleSummary() (Action, string) {
	if s.selected == nil {
		return Action{}, "select a building first"
	}
	if !summarisable(s.selected.Session) {
		return Action{}, "no summary for " + s.selected.Session.Harness + " sessions"
	}
	if s.summaryOpen() {
		return s.Act(ActionSummaryHide)
	}
	return s.Act(ActionSummary)
}

// summarisable is whether a session's transcript is Claude Code's, the
// only kind that carries a recap or that the summariser can read.
func summarisable(session state.Session) bool {
	return session.Harness == "" || session.Harness == "claude"
}

func (s *Scene) summaryOpen() bool {
	return s.selected != nil && s.summaryFor == s.selected.Session.ID
}

// summaryActions is the summary's part of the selection's actions:
// the toggle, and Generate when the card is open on a session with no
// recap of its own and nothing already running.
func (s *Scene) summaryActions() []ActionKind {
	session := s.selected.Session
	if !summarisable(session) {
		return nil
	}
	if !s.summaryOpen() {
		return []ActionKind{ActionSummary}
	}
	kinds := []ActionKind{ActionSummaryHide}
	if session.Recap.Text == "" && !s.generated[session.ID].Pending {
		kinds = append(kinds, ActionGenerate)
	}
	return kinds
}

// summary is the open summary's lines and paragraph. Claude Code's own
// recap comes first, shown as written with its age; without one, a
// generated summary, else the last prompt, each labelled as what it is.
func (s *Scene) summary(session state.Session) ([]string, string) {
	if r := session.Recap; r.Text != "" {
		return []string{"recap    " + format.Age(s.city.Time.Sub(r.At)) + " old, " + since(r.PromptsSince)}, r.Text
	}
	g := s.generated[session.ID]
	switch {
	case g.Pending:
		return []string{"summary  generating with " + claude.SummaryModel + "…"}, ""
	case g.Err != "":
		return []string{"summary  failed: " + g.Err}, ""
	case g.Text != "":
		return []string{"summary  generated " + format.Age(s.city.Time.Sub(g.At)) + " ago with " + claude.SummaryModel}, g.Text
	case session.LastPrompt != "":
		return []string{"recap    none yet; the last prompt was"}, session.LastPrompt
	}
	return []string{"recap    none yet"}, ""
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
