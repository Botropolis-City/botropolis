package city

import (
	"time"

	"botropolis.city/botropolis/pkg/state"
)

// The building works. Item 89.
//
// The building is the session, so nothing else stands for it. While it
// is mid-turn its roof is scaffolded, with work lights, and each tool
// call the main thread makes runs one load up a hoist on its outside
// corner to the scaffold, which is where tool calls show at a glance.
// When the turn ends the scaffold comes down. At still a load is a flash
// of the work lights, so no level loses the reading.
const (
	LoadFor         = 1500 * time.Millisecond
	LoadFlashFor    = 300 * time.Millisecond
	ScaffoldDownFor = 800 * time.Millisecond
)

// Load is one tool call on its way up a building's hoist, P of the way
// from the ground to the scaffold, or a flash of the work lights at
// still. Merged is the last of a burst too close to show one by one,
// drawn heavier.
type Load struct {
	P      float64
	Flash  bool
	Merged bool
}

// Scaffold is how far up a building's roof scaffold is: all the way
// while it is mid-turn, coming down for ScaffoldDownFor after the turn
// ends, and not there otherwise.
func (s *Scene) Scaffold(b *Building) float64 {
	if b == nil || b.Blinds {
		return 0
	}
	// A building still going up has its site's scaffolding; the roof's
	// waits until it stands (item 109).
	if s.Rising(b.Session.ID) < 1 {
		return 0
	}
	if b.Session.State == state.Working {
		return 1
	}
	ended, ok := s.struck[b.Session.ID]
	if !ok {
		return 0
	}
	p := float64(s.now().Sub(ended)) / float64(ScaffoldDownFor)
	if p >= 1 {
		delete(s.struck, b.Session.ID)
		return 0
	}
	return 1 - max(0, p)
}

// ReadEvery is how often the map is sent a snapshot, used to spread a
// first interval's pulses; later ones use the time since the last.
const ReadEvery = 2 * time.Second

// PulseGap is the closest two loads start. Calls that would come closer
// merge into the last one, which is drawn brighter.
const PulseGap = 100 * time.Millisecond

type load struct {
	since  time.Time
	merged bool
}

// noteLoads sends one load for every tool call the main thread made
// since the last snapshot, spread evenly across the time between the two
// so each reads on its own, merging only those that would come closer
// than PulseGap. The first snapshot only takes note.
func (s *Scene) noteLoads(snapshot state.Snapshot, now time.Time) {
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
		working := sess.State == state.Working
		if s.working[sess.ID] && !working {
			if s.struck == nil {
				s.struck = map[string]time.Time{}
			}
			s.struck[sess.ID] = now
		}
		if s.working == nil {
			s.working = map[string]bool{}
		}
		s.working[sess.ID] = working
		was, known := s.calls[sess.ID]
		s.calls[sess.ID] = sess.ToolCalls
		n := sess.ToolCalls - was
		if first || !known || n <= 0 {
			continue
		}
		fit := max(1, int(interval/PulseGap))
		merged := n > fit
		n = min(n, fit)
		if s.loads == nil {
			s.loads = map[string][]load{}
		}
		for k := 0; k < n; k++ {
			at := now.Add(time.Duration(k) * interval / time.Duration(n))
			s.loads[sess.ID] = append(s.loads[sess.ID], load{since: at, merged: merged && k == n-1})
		}
	}
	for id := range s.calls {
		if !here[id] {
			delete(s.calls, id)
			delete(s.loads, id)
			delete(s.working, id)
			delete(s.struck, id)
		}
	}
}

func (s *Scene) loadFor() time.Duration {
	if s.reduced {
		return LoadFlashFor
	}
	return LoadFor
}

// Loads is every load on a session's hoist now; those still to start
// wait their turn.
func (s *Scene) Loads(id string) []Load {
	now, span := s.now(), s.loadFor()
	var out []Load
	kept := s.loads[id][:0]
	for _, p := range s.loads[id] {
		t := float64(now.Sub(p.since)) / float64(span)
		if t >= 1 {
			continue
		}
		kept = append(kept, p)
		if t >= 0 {
			out = append(out, Load{P: t, Flash: s.reduced, Merged: p.merged})
		}
	}
	if len(kept) == 0 {
		delete(s.loads, id)
	} else {
		s.loads[id] = kept
	}
	return out
}

// hoisting is whether a load climbs or a scaffold comes down.
func (s *Scene) hoisting() bool {
	for id := range s.loads {
		s.Loads(id)
	}
	for id, ended := range s.struck {
		if s.now().Sub(ended) >= ScaffoldDownFor {
			delete(s.struck, id)
		}
	}
	return len(s.loads) > 0 || len(s.struck) > 0
}
