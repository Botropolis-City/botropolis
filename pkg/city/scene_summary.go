package city

import "botropolis.city/botropolis/pkg/state"

// Summary is the city's tally as triage sees it: a session needing you
// that was marked done, or is snoozed, counts as set aside rather than
// needing you, the way botropolis bar counts it (item 189).
func (s *Scene) Summary() Summary {
	sum := s.City().Summary()
	for _, b := range s.city.Buildings() {
		if !b.Packed && s.setAside(b) {
			sum.NeedsYou--
			sum.SetAside++
		}
	}
	return sum
}

// StateCard lists the sessions in a state for hovering its strip chip,
// leaving out the ones triage has set aside.
func (s *Scene) StateCard(st state.State) Card {
	return s.City().stateCard(st, func(b *Building) bool { return !s.setAside(b) })
}

// setAside is whether a building needs you but triage has put it down.
func (s *Scene) setAside(b *Building) bool {
	return b.Session.State == state.NeedsYou && s.layout.SetAside(b.Session, s.now())
}

// needingYou is how many sessions need you and are not set aside.
func (s *Scene) needingYou() int {
	if s.city == nil {
		return 0
	}
	return s.Summary().NeedsYou
}
