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
type Drag struct {
	District  *District
	At        Point
	Target    Rect
	Droppable bool
}

type districtDrag struct {
	root string
	at   Point
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
	s.drag = &districtDrag{root: hit.District.Root, at: s.camera.ScreenToWorld(screen)}
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
	var d *District
	for _, district := range s.city.Districts {
		if district.Root == s.drag.root {
			d = district
		}
	}
	if d == nil {
		return Drag{}, false
	}
	drag := Drag{District: d, At: s.drag.at}
	if slot, ok := s.city.DropSlot(s.drag.at); ok {
		drag.Target = s.city.SlotRect(slot)
		drag.Droppable = true
	} else {
		cell := CellOf(s.drag.at)
		drag.Target = cell.Rect()
	}
	return drag, true
}

// CancelDistrictDrag puts the district down where it was.
func (s *Scene) CancelDistrictDrag() { s.drag = nil }

// DropDistrict ends the drag, and moves the district to the slot under
// the pointer if it will take it. The city is rebuilt on the new slots
// at once, so the drop lands without waiting for the next snapshot.
func (s *Scene) DropDistrict(screen Point) bool {
	if s.drag == nil {
		return false
	}
	root := s.drag.root
	s.drag = nil
	slot, ok := s.city.DropSlot(s.camera.ScreenToWorld(screen))
	if !ok || s.layout == nil || !s.layout.MoveDistrict(root, slot, s.city.Rings()) {
		return false
	}
	s.SetSnapshot(s.snapshot)
	return true
}
