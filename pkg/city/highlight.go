package city

// Contextual highlight: what the thing under the pointer is tied to.
//
// It is the answer to a question colour cannot carry. Thirteen MCP
// servers cannot be thirteen hues — the map caps at three — so Servers
// is legible a different way: put the pointer on a tower and the
// sessions that call that server stay lit while everything else steps
// back. "What breaks if this goes down" is a set, and a set is better
// shown than coloured.
//
// It rides the same machinery a view does. Receding is what the renderer
// already knows how to do; this only decides which objects are spared,
// which is why it works in Attention as well as inside a view.

// Highlight is the lit set. Active is false when the pointer is on
// nothing, which is the common case and must cost nothing.
type Highlight struct {
	Active    bool
	buildings map[*Building]bool
	towers    map[*Tower]bool
}

func (h Highlight) HasBuilding(b *Building) bool { return h.buildings[b] }
func (h Highlight) HasTower(t *Tower) bool       { return h.towers[t] }

// Highlight is what the pointer, or failing that the selection, ties
// together. Hover wins over selection: the pointer is the live question
// and the selection is the one you parked.
func (s *Scene) Highlight() Highlight {
	h := Highlight{buildings: map[*Building]bool{}, towers: map[*Tower]bool{}}
	if s.focus != "" {
		h.Active = true
		for _, b := range s.city.Buildings() {
			if b.Group != nil && b.Group.ID == s.focus {
				h.buildings[b] = true
			}
		}
		return h
	}
	switch hit := s.hover; {
	case hit.Tower != nil:
		h.Active = true
		h.towers[hit.Tower] = true
		for _, b := range s.city.Buildings() {
			if b.Session.MCPCalls[hit.Tower.Server.Name] > 0 {
				h.buildings[b] = true
			}
		}
	case hit.Building != nil:
		s.tieBuilding(&h, hit.Building)
	case hit.District != nil:
		h.Active = true
		for _, b := range s.city.Buildings() {
			if s.city.DistrictOf(b) == hit.District {
				h.buildings[b] = true
			}
		}
	default:
		if s.selected != nil {
			s.tieBuilding(&h, s.selected)
		}
	}
	return h
}

// tieBuilding lights a session and the servers it leans on.
//
// A session that calls nothing ties nothing, and the highlight stays
// down. Otherwise every click on a building would fade the whole city
// to answer a question with no answer in it — and a click on a building
// is how you read its card, which is most of the time.
func (s *Scene) tieBuilding(h *Highlight, b *Building) {
	for _, t := range s.city.Towers {
		if b.Session.MCPCalls[t.Server.Name] > 0 {
			h.towers[t] = true
		}
	}
	if len(h.towers) == 0 {
		return
	}
	h.Active = true
	h.buildings[b] = true
}
