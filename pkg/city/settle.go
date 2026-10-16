package city

import "time"

// Settle is a building changing between live and stopped, drawn once as
// it happens: its lights go out storey by storey and its worker goes
// home, or the reverse, quicker. Under reduced motion either is a short
// cross-fade. Item 86.
type Settle int

const (
	SettleNone Settle = iota
	SettleRest
	SettleWake
)

const (
	RestFor       = 1500 * time.Millisecond
	WakeFor       = 750 * time.Millisecond
	SettleFadeFor = 300 * time.Millisecond
)

type settling struct {
	kind  Settle
	since time.Time
}

// noteSettling starts a transition for every building that was live at
// the last snapshot and is stopped at this one, or the reverse. A
// building the last snapshot did not have starts none.
func (s *Scene) noteSettling(now time.Time) {
	awake := map[string]bool{}
	for _, b := range s.city.Buildings() {
		id := b.Session.ID
		awake[id] = !b.Blinds
		was, known := s.awake[id]
		if !known || was == awake[id] {
			continue
		}
		if s.settling == nil {
			s.settling = map[string]settling{}
		}
		kind := SettleWake
		if was {
			kind = SettleRest
		}
		s.settling[id] = settling{kind: kind, since: now}
	}
	s.awake = awake
}

// Settling is the transition a session's building is in and how far it
// has got, from 0 to 1, or SettleNone.
func (s *Scene) Settling(id string) (Settle, float64) {
	t, ok := s.settling[id]
	if !ok {
		return SettleNone, 0
	}
	p := float64(s.now().Sub(t.since)) / float64(s.settleFor(t.kind))
	if p >= 1 {
		delete(s.settling, id)
		return SettleNone, 0
	}
	return t.kind, max(0, p)
}

func (s *Scene) settleFor(kind Settle) time.Duration {
	switch {
	case s.reduced:
		return SettleFadeFor
	case kind == SettleWake:
		return WakeFor
	}
	return RestFor
}

func (s *Scene) settlingAny() bool {
	now := s.now()
	for _, t := range s.settling {
		if now.Sub(t.since) < s.settleFor(t.kind) {
			return true
		}
	}
	return false
}
