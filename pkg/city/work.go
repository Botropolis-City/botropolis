package city

import (
	"time"

	"botropolis.city/botropolis/pkg/state"
)

// The building works. Item 89.
//
// The building is the session, so nothing else stands for it. While it
// is mid-turn one floor, the one at its context fill, glows brighter than
// its lit windows. Each tool call the main thread makes sends one pulse
// up the face to the roof, which is where tool calls show at a glance.
// At still the pulse is a brief brightening, so no level loses the
// reading.
const (
	PulseFor     = 600 * time.Millisecond
	PulseFadeFor = 300 * time.Millisecond
)

// Pulse is one tool call on its way up a building's face, P of the way
// from the working floor to the roof, or a brightening at still.
type Pulse struct {
	P        float64
	Brighten bool
}

// WorkingFloor is the height of a building's working floor, as a share
// of its height, and whether it has one: only while it is mid-turn.
func (s *Scene) WorkingFloor(b *Building) (float64, bool) {
	if b == nil || b.Session.State != state.Working || b.Blinds {
		return 0, false
	}
	return min(1, max(0.1, b.Fill)), true
}

// notePulses sends a pulse for every tool call that started since the
// last snapshot: a tool where there was none, or a different one. The
// first snapshot only takes note.
func (s *Scene) notePulses(snapshot state.Snapshot, now time.Time) {
	first := s.tools == nil
	if first {
		s.tools = map[string]string{}
	}
	here := map[string]bool{}
	for _, sess := range snapshot.Sessions {
		here[sess.ID] = true
		was, known := s.tools[sess.ID]
		s.tools[sess.ID] = sess.Tool
		if first || !known || sess.Tool == "" || sess.Tool == was {
			continue
		}
		if s.pulses == nil {
			s.pulses = map[string][]time.Time{}
		}
		s.pulses[sess.ID] = append(s.pulses[sess.ID], now)
	}
	for id := range s.tools {
		if !here[id] {
			delete(s.tools, id)
			delete(s.pulses, id)
		}
	}
}

func (s *Scene) pulseFor() time.Duration {
	if s.reduced {
		return PulseFadeFor
	}
	return PulseFor
}

// Pulses is every pulse running on a session's building.
func (s *Scene) Pulses(id string) []Pulse {
	now, span := s.now(), s.pulseFor()
	var out []Pulse
	kept := s.pulses[id][:0]
	for _, since := range s.pulses[id] {
		p := float64(now.Sub(since)) / float64(span)
		if p >= 1 {
			continue
		}
		kept = append(kept, since)
		out = append(out, Pulse{P: max(0, p), Brighten: s.reduced})
	}
	if len(kept) == 0 {
		delete(s.pulses, id)
	} else {
		s.pulses[id] = kept
	}
	return out
}

func (s *Scene) pulsing() bool {
	for id := range s.pulses {
		if len(s.Pulses(id)) > 0 {
			return true
		}
	}
	return false
}
