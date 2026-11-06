package city

// The tower_lines setting (item 347). A packed session kept its beam
// from every tower it ever called, so a depot of ninety crates had thin
// lines across the whole empty south of the map, and packing a session
// did nothing to thin the view out. The towers' beams run to the
// buildings on the map unless the setting asks for the crates too, and
// the Connections lens's panel is where it is switched.

// SetDepotBeams runs the towers' beams to the crates in the depot as
// well, or to the buildings on the map only. It applies to the city
// already built as well as to the next.
func (s *Scene) SetDepotBeams(on bool) {
	if s.depotBeams == on {
		return
	}
	s.depotBeams = on
	if s.city != nil {
		s.city.depotBeams = on
	}
	// The beams are painted into the kept ground, so the ground moves
	// with the list, as it does for the detail setting.
	s.generation++
	s.groundGeneration++
}

// DepotBeams is whether the towers' beams run to the depot's crates.
func (s *Scene) DepotBeams() bool { return s.depotBeams }

// adopt hands a newly built city the scene's settings that the city
// reads, so every build site cannot forget one.
func (s *Scene) adopt(c *City) *City {
	c.depotBeams = s.depotBeams
	return c
}
