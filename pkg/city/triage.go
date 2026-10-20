package city

import (
	"errors"
	"slices"
	"time"

	"botropolis.city/botropolis/pkg/state"
)

// Triage walks what is your move, longest-waiting first (item 88). Each
// step selects the session and centres the camera on it, so its card
// opens. Today that is the sessions needing you; reviews, failed checks
// and approvals waiting for a merge join when Botropolis has them.

// TriageStep is where the walk is: the session, and which of how many.
type TriageStep struct {
	Building     *Building
	Index, Count int
}

// triageDone is what d asks the actor to do beside leaving the walk: it
// packs the session up (item 86).
const triageDone = ActionPack

// TriageWalk is every step of the walk, longest-waiting first: what
// needs you, less what was done and has not moved since, and what is
// snoozed.
func (s *Scene) TriageWalk() []*Building {
	var walk []*Building
	for _, b := range s.city.Buildings() {
		if s.inTriage(b) {
			walk = append(walk, b)
		}
	}
	slices.SortFunc(walk, func(a, b *Building) int {
		return state.LongestWaiting(a.Session, b.Session)
	})
	return walk
}

func (s *Scene) inTriage(b *Building) bool {
	sess := b.Session
	if sess.State != state.NeedsYou {
		return false
	}
	if s.layout == nil {
		return true
	}
	if at, ok := s.layout.Done[sess.ID]; ok && !sess.LastActivity.After(at) {
		return false
	}
	if until, ok := s.layout.Snoozed[sess.ID]; ok && s.now().Before(until) {
		return false
	}
	return true
}

// StartTriage takes the walk's first step, and reports false when
// nothing waits on you.
func (s *Scene) StartTriage() bool {
	walk := s.TriageWalk()
	if len(walk) == 0 {
		s.triageAt = ""
		return false
	}
	s.step(walk[0])
	return true
}

func (s *Scene) step(b *Building) {
	s.triageAt = b.Session.ID
	s.Select(b.Session.ID)
}

// TriageStep is the step the walk is on.
func (s *Scene) TriageStep() (TriageStep, bool) {
	walk := s.TriageWalk()
	for i, b := range walk {
		if b.Session.ID == s.triageAt {
			return TriageStep{Building: b, Index: i, Count: len(walk)}, true
		}
	}
	return TriageStep{}, false
}

// TriageNext moves on, back to the first after the last, and reports
// false when the walk is empty.
func (s *Scene) TriageNext() bool {
	walk := s.TriageWalk()
	if len(walk) == 0 {
		s.triageAt = ""
		return false
	}
	i := slices.IndexFunc(walk, func(b *Building) bool { return b.Session.ID == s.triageAt })
	s.step(walk[(i+1)%len(walk)])
	return true
}

// leave takes the current step out of the walk with mark, and moves on
// to the one after it.
func (s *Scene) leave(mark func(id string)) {
	walk := s.TriageWalk()
	i := slices.IndexFunc(walk, func(b *Building) bool { return b.Session.ID == s.triageAt })
	if i < 0 {
		return
	}
	mark(s.triageAt)
	rest := slices.Delete(slices.Clone(walk), i, i+1)
	if len(rest) == 0 {
		s.triageAt = ""
		return
	}
	s.step(rest[i%len(rest)])
}

// TriageDone marks the step done: it leaves the walk until the session
// does something new.
func (s *Scene) TriageDone() Action {
	step, ok := s.TriageStep()
	if !ok || s.layout == nil {
		return Action{}
	}
	at := step.Building.Session.LastActivity
	s.leave(func(id string) {
		if s.layout.Done == nil {
			s.layout.Done = map[string]time.Time{}
		}
		s.layout.Done[id] = at
	})
	if triageDone == ActionNone {
		return Action{}
	}
	return Action{Kind: triageDone, SessionID: step.Building.Session.ID}
}

// TriageSnooze takes the step out of the walk until a time, which must
// be later than now.
func (s *Scene) TriageSnooze(until time.Time) error {
	if !until.After(s.now()) {
		return errors.New("that time has passed")
	}
	if _, ok := s.TriageStep(); !ok || s.layout == nil {
		return errors.New("nothing to snooze")
	}
	s.leave(func(id string) {
		if s.layout.Snoozed == nil {
			s.layout.Snoozed = map[string]time.Time{}
		}
		s.layout.Snoozed[id] = until
	})
	return nil
}

// TriageRest asks to rest the step's session: to stop it, which keeps
// it on its lot.
func (s *Scene) TriageRest() (Action, string) {
	if _, ok := s.TriageStep(); !ok {
		return Action{}, "nothing to rest"
	}
	return s.Act(ActionStop)
}

// TriageAct does the step's card's first action.
func (s *Scene) TriageAct() (Action, string) {
	if _, ok := s.TriageStep(); !ok {
		return Action{}, "nothing to act on"
	}
	kinds := s.Actions()
	if len(kinds) == 0 {
		return Action{}, ""
	}
	return s.Act(kinds[0])
}
