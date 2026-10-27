package city

import (
	"slices"

	"botropolis.city/botropolis/pkg/state"
)

// LayoutsDone is how many plans the scene has made, and how many
// groups' orders it has read, rather than remembered (item 197).
func (s *Scene) LayoutsDone() (plans, orders int) {
	if s.memo == nil {
		return 0, 0
	}
	return s.memo.plansMade, s.memo.orders.Reads
}

// StreetsRouted is how many walks along the avenues the scene has
// routed, rather than remembered (item 206).
func (s *Scene) StreetsRouted() int {
	if s.memo == nil {
		return 0
	}
	return s.memo.walksRouted
}

// StreetsLaid is how many times the scene has laid road on the
// corridors, rather than remembered it (item 206).
func (s *Scene) StreetsLaid() int {
	if s.memo == nil {
		return 0
	}
	return s.memo.laysDone
}

// StaleWalks is how many remembered walks are from a plan other than
// the one the city stands on.
func (s *Scene) StaleWalks() int {
	if s.memo == nil {
		return 0
	}
	stale := 0
	for k := range s.memo.walks {
		if k.plan != s.city.planID {
			stale++
		}
	}
	return stale
}

// GroundOf is the ground a city's standing layer draws (item 206).
func GroundOf(c *City) any { return groundOf(c) }

// Ground is the ground the scene last compared.
func (s *Scene) Ground() any { return s.ground }

// FromScratch is the scene's snapshot built from nothing, facing the
// way the scene does.
func (s *Scene) FromScratch() *City {
	c := build(s.snapshot, s.layout, s.ladder, false, s.ledger, nil, s.offMap)
	c.Face(s.camera.Heading)
	return c
}

// BridgeRoads is the road laid only to reach a bridge's landing
// (item 247).
func (c *City) BridgeRoads() []Cell { return c.bridgeRoads }

// DistrictsLookedAt is how many districts DistrictOf has looked through
// to find buildings' districts (item 270).
func (c *City) DistrictsLookedAt() int { return c.districtLooks }

// TriageWalkEveryBuilding is the triage walk taken over every building
// in the city, as it was before item 270: the oracle the walk is held to.
func (s *Scene) TriageWalkEveryBuilding() []*Building {
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

// TriageLooks is how many buildings the triage walk has looked at.
func (s *Scene) TriageLooks() int { return s.triageLooks }

// ClinicLooks is how many buildings the clinic's count has looked at.
func (s *Scene) ClinicLooks() int { return s.clinicLooks }

// ClinicCountEveryBuilding is the clinic's count by walking every
// building in the city, the oracle the indexed count is held to.
func (s *Scene) ClinicCountEveryBuilding() int {
	n := 0
	for _, b := range s.city.Buildings() {
		if s.inTriage(b) {
			n++
			continue
		}
		if _, _, ok := troubleOf(b, s.now()); ok {
			n++
		}
	}
	return n
}
