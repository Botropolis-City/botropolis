package city

import "botropolis.city/botropolis/pkg/plan"

// Dragging a district to another slot. Only the remembered slots move:
// the mover's, and on a taken slot the holder's, which trades places
// with it. Nothing else here moves a district. Item 85.

// MoveDistrict remembers a district on another slot, swapping with
// whatever district remembers that slot. The plaza's slot and anything
// past the ring just outside the city's rings are refused, as is a
// district the layout has no slot for.
func (l *Layout) MoveDistrict(root string, to plan.Slot, rings int) bool {
	mem := l.memory()
	from, ok := mem.Slots[root]
	if !ok || to.Ring() < 1 || to.Ring() > rings+1 {
		return false
	}
	if to == from {
		return true
	}
	for other, slot := range mem.Slots {
		if slot == to {
			mem.Slots[other] = from
		}
	}
	mem.Slots[root] = to
	return true
}

// MoveDistricts moves a group of districts by one offset, keeping their
// places relative to each other. Unlike a single move it swaps with
// nothing: a target held by a district outside the group, on the plaza
// or past the ring outside the city refuses the whole move. Item 283.
func (l *Layout) MoveDistricts(roots []string, by plan.Slot, rings int) bool {
	to, ok := l.groupTargets(roots, by, rings)
	if !ok {
		return false
	}
	mem := l.memory()
	for root, slot := range to {
		mem.Slots[root] = slot
	}
	return true
}

func (l *Layout) groupTargets(roots []string, by plan.Slot, rings int) (map[string]plan.Slot, bool) {
	mem := l.memory()
	to := map[string]plan.Slot{}
	for _, root := range roots {
		from, ok := mem.Slots[root]
		slot := plan.Slot{Col: from.Col + by.Col, Row: from.Row + by.Row}
		if !ok || slot.Ring() < 1 || slot.Ring() > rings+1 {
			return nil, false
		}
		to[root] = slot
	}
	if len(to) == 0 {
		return nil, false
	}
	for other, slot := range mem.Slots {
		if _, moving := to[other]; moving {
			continue
		}
		for _, target := range to {
			if target == slot {
				return nil, false
			}
		}
	}
	return to, true
}

// Rings is how many rings of slots lie round the plaza.
func (c *City) Rings() int { return c.plan.Rings() }

// SlotRect is the ground a slot covers, out to the ring just past the
// city's, or an empty rect further out.
func (c *City) SlotRect(s plan.Slot) Rect {
	b, ok := c.plan.SlotBlock(s)
	if !ok {
		return Rect{}
	}
	return cellRect(b)
}

// DropSlot is the slot a district dropped at a world point would take,
// or false for the plaza, an avenue, or past the ring outside the city.
func (c *City) DropSlot(p Point) (plan.Slot, bool) {
	cell := CellOf(p)
	s, ok := c.plan.SlotAt(plan.Cell{Col: cell.Col, Row: cell.Row})
	if !ok || s.Ring() < 1 {
		return plan.Slot{}, false
	}
	return s, true
}

// Drag is a district on the move: where the pointer holds it, and the
// slot under the pointer, outlined, and whether it would take the drop.
// Members are every district carried, the held one first, and Targets
// the slot each would land on (item 283).
type Drag struct {
	District  *District
	At        Point
	Target    Rect
	Droppable bool
	Members   []*District
	Targets   []Rect
}

type districtDrag struct {
	roots []string
	at    Point
}

// PressDistrict starts dragging the district whose floor or name plate
// is under a press. A press on a building or on open ground does not,
// and neither does any press while the pan modifier is held: the caller
// pans instead.
func (s *Scene) PressDistrict(screen Point, panHeld bool) bool {
	s.drag = nil
	if panHeld {
		return false
	}
	hit := s.hover
	if hit.District == nil || hit.Building != nil {
		s.PointerMove(screen)
		hit = s.hover
	}
	if hit.District == nil || hit.Building != nil {
		return false
	}
	if _, ok := s.city.plan.Slots[hit.District.Root]; !ok {
		return false
	}
	roots := []string{hit.District.Root}
	if s.chosen[hit.District.Root] {
		for _, root := range s.SelectedDistricts() {
			if root != hit.District.Root {
				roots = append(roots, root)
			}
		}
	}
	s.drag = &districtDrag{roots: roots, at: s.camera.ScreenToWorld(screen)}
	return true
}

// DragDistrict follows the pointer with the district being dragged.
func (s *Scene) DragDistrict(screen Point) {
	if s.drag != nil {
		s.drag.at = s.camera.ScreenToWorld(screen)
	}
}

// DistrictDrag is the district being dragged, if one is.
func (s *Scene) DistrictDrag() (Drag, bool) {
	if s.drag == nil {
		return Drag{}, false
	}
	byRoot := map[string]*District{}
	for _, district := range s.city.Districts {
		byRoot[district.Root] = district
	}
	drag := Drag{At: s.drag.at}
	for _, root := range s.drag.roots {
		if d := byRoot[root]; d != nil {
			drag.Members = append(drag.Members, d)
		}
	}
	if len(drag.Members) == 0 || drag.Members[0].Root != s.drag.roots[0] {
		return Drag{}, false
	}
	drag.District = drag.Members[0]
	slot, ok := s.city.DropSlot(s.drag.at)
	if !ok {
		drag.Target = CellOf(s.drag.at).Rect()
		drag.Targets = []Rect{drag.Target}
		return drag, true
	}
	drag.Target = s.city.SlotRect(slot)
	if len(s.drag.roots) == 1 {
		drag.Targets = []Rect{drag.Target}
		drag.Droppable = true
		return drag, true
	}
	by := s.dragOffset(slot)
	for _, d := range drag.Members {
		from := s.city.plan.Slots[d.Root]
		drag.Targets = append(drag.Targets, s.city.SlotRect(plan.Slot{Col: from.Col + by.Col, Row: from.Row + by.Row}))
	}
	if s.layout != nil {
		_, drag.Droppable = s.layout.groupTargets(s.drag.roots, by, s.city.Rings())
	}
	return drag, true
}

// dragOffset is how far a drop on a slot moves the held district.
func (s *Scene) dragOffset(to plan.Slot) plan.Slot {
	from := s.city.plan.Slots[s.drag.roots[0]]
	return plan.Slot{Col: to.Col - from.Col, Row: to.Row - from.Row}
}

// CancelDistrictDrag puts the district down where it was.
func (s *Scene) CancelDistrictDrag() { s.drag = nil }

// DropDistrict ends the drag, and moves the district to the slot under
// the pointer if it will take it, or the whole group carried by the same
// offset. The city is rebuilt on the new slots at once, so the drop
// lands without waiting for the next snapshot.
func (s *Scene) DropDistrict(screen Point) bool {
	if s.drag == nil {
		return false
	}
	defer func() { s.drag = nil }()
	slot, ok := s.city.DropSlot(s.camera.ScreenToWorld(screen))
	if !ok || s.layout == nil {
		return false
	}
	roots := s.drag.roots
	if len(roots) == 1 {
		ok = s.layout.MoveDistrict(roots[0], slot, s.city.Rings())
	} else {
		ok = s.layout.MoveDistricts(roots, s.dragOffset(slot), s.city.Rings())
	}
	if !ok {
		return false
	}
	s.restate()
	return true
}
