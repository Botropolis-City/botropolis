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
// from the working floor to the roof, or a brightening at still. Merged
// is the last of a burst too close to show one by one, drawn brighter.
type Pulse struct {
	P        float64
	Brighten bool
	Merged   bool
}

// WorkingFloor is the height of a building's working floor, as a share
// of its height, and whether it has one: only while it is mid-turn.
func (s *Scene) WorkingFloor(b *Building) (float64, bool) {
	if b == nil || b.Session.State != state.Working || b.Blinds {
		return 0, false
	}
	return min(1, max(0.1, b.Fill)), true
}

// ReadEvery is how often the map is sent a snapshot, used to spread a
// first interval's pulses; later ones use the time since the last.
const ReadEvery = 2 * time.Second

// PulseGap is the closest two pulses start. Calls that would come closer
// merge into the last one, which is drawn brighter.
const PulseGap = 100 * time.Millisecond

type pulse struct {
	since  time.Time
	merged bool
}

// notePulses sends one pulse for every tool call the main thread made
// since the last snapshot, spread evenly across the time between the two
// so each reads on its own, merging only those that would come closer
// than PulseGap. The first snapshot only takes note.
func (s *Scene) notePulses(snapshot state.Snapshot, now time.Time) {
	first := s.calls == nil
	if first {
		s.calls = map[string]int{}
	}
	interval := ReadEvery
	if !s.lastRead.IsZero() && now.After(s.lastRead) {
		interval = now.Sub(s.lastRead)
	}
	s.lastRead = now
	here := map[string]bool{}
	for _, sess := range snapshot.Sessions {
		here[sess.ID] = true
		was, known := s.calls[sess.ID]
		s.calls[sess.ID] = sess.ToolCalls
		n := sess.ToolCalls - was
		if first || !known || n <= 0 {
			continue
		}
		fit := max(1, int(interval/PulseGap))
		merged := n > fit
		n = min(n, fit)
		if s.pulses == nil {
			s.pulses = map[string][]pulse{}
		}
		for k := 0; k < n; k++ {
			at := now.Add(time.Duration(k) * interval / time.Duration(n))
			s.pulses[sess.ID] = append(s.pulses[sess.ID], pulse{since: at, merged: merged && k == n-1})
		}
	}
	for id := range s.calls {
		if !here[id] {
			delete(s.calls, id)
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

// Pulses is every pulse running on a session's building now; those still
// to start wait their turn.
func (s *Scene) Pulses(id string) []Pulse {
	now, span := s.now(), s.pulseFor()
	var out []Pulse
	kept := s.pulses[id][:0]
	for _, p := range s.pulses[id] {
		t := float64(now.Sub(p.since)) / float64(span)
		if t >= 1 {
			continue
		}
		kept = append(kept, p)
		if t >= 0 {
			out = append(out, Pulse{P: t, Brighten: s.reduced, Merged: p.merged})
		}
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
		s.Pulses(id)
	}
	return len(s.pulses) > 0
}
