package city

import "strings"

// StirringStates names every timed state Stirring covers, each one a
// thing a session's building does with the clock alone: a tug bringing
// it in, and its build after; a truck hauling it away; settling to rest
// or waking; celebrating; pouring a course; a load on its hoist; a flag
// going up or down; its scaffold coming down (item 212).
var StirringStates = []string{"arriving", "hauled", "settling", "celebrating", "pouring", "loads", "hoisting", "struck"}

// Stirring is every session one of whose timed states runs now, into
// the map given, emptied first, or a new one. A state that has just run
// out may still be in it until it is next asked, as its own accessor
// prunes it: that costs a quiet frame, never a stale one.
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
