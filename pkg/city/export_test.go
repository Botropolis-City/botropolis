package city

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

// FromScratch is the scene's snapshot built from nothing, facing the
// way the scene does.
func (s *Scene) FromScratch() *City {
	c := build(s.snapshot, s.layout, s.ladder, false, s.ledger, nil)
	c.Face(s.camera.Heading)
	return c
}
