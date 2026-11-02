package city

import (
	"slices"
	"strings"
)

// StirringStates names every timed state Stirring covers, each one a
// thing a session's building does with the clock alone: a tug bringing
// it in, and its build after; a truck hauling it away; settling to rest
// or waking; celebrating; pouring a course; a load on its hoist; a flag
// going up or down; its scaffold coming down (item 212).
var StirringStates = []string{"arriving", "hauled", "settling", "celebrating", "pouring", "loads", "hoisting", "struck"}

// Stirring is every session one of whose timed states runs now, into
// the map given, emptied first, or a new one. A state that has just run
// out may still be in it until the scene next ticks, which lets go of it
// (item 294): that costs a quiet frame, never a stale one.
func (s *Scene) Stirring(into map[string]bool) map[string]bool {
	if into == nil {
		into = map[string]bool{}
	}
	clear(into)
	now := s.now()
	for _, v := range s.voyages {
		if v.Kind == Arrival {
			into[v.SessionID] = true
		}
	}
	for _, h := range s.hauls {
		if now.Sub(h.Since) < h.For {
			into[h.SessionID] = true
		}
	}
	for id := range s.settling {
		into[id] = true
	}
	for id := range s.celebrating {
		into[id] = true
	}
	for id := range s.jobs {
		into[id] = true
	}
	for id, loads := range s.loads {
		if len(loads) > 0 {
			into[id] = true
		}
	}
	for key, h := range s.hoists {
		if now.Sub(h.since) < FlagFor {
			id, _, _ := strings.Cut(key, " ")
			into[id] = true
		}
	}
	for id := range s.struck {
		into[id] = true
	}
	return into
}

// Expire lets go of every timed state that has run out, on the scene's
// tick and as a frame begins, so no read of a frame changes what the
// rest of it, or the next, reads (item 294). It walks only the states
// that are running.
func (s *Scene) Expire() {
	now := s.now()
	for id, ended := range s.struck {
		if now.Sub(ended) >= ScaffoldDownFor {
			delete(s.struck, id)
		}
	}
	span := s.loadFor()
	for id, loads := range s.loads {
		kept := loads[:0]
		for _, p := range loads {
			if now.Sub(p.since) < span {
				kept = append(kept, p)
			}
		}
		if len(kept) == 0 {
			delete(s.loads, id)
		} else {
			s.loads[id] = kept
		}
	}
	for id, t := range s.settling {
		if now.Sub(t.since) >= s.settleFor(t.kind) {
			delete(s.settling, id)
		}
	}
	for id, at := range s.celebrating {
		if now.Sub(at) >= CelebrateFor {
			delete(s.celebrating, id)
		}
	}
	for id, j := range s.jobs {
		if t := now.Sub(j.since); t >= CompactionFor || t < 0 {
			delete(s.jobs, id)
		}
	}
	s.voyages = slices.DeleteFunc(s.voyages, func(v *Voyage) bool { return s.voyageDone(v, now) })
	s.hauls = slices.DeleteFunc(s.hauls, func(h Haul) bool { return now.Sub(h.Since) >= h.For })
}
