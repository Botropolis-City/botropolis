package city

import "botropolis.city/botropolis/pkg/state"

// Whether anything on the map is moving under its own steam.
//
// The renderer paints at a tick, and a tick faster than the eye needs
// is work thrown away. Nothing here is about what the map means — it is
// about whether the next frame could differ from this one — so the
// answer lists every object that moves between frames and nothing else.
//
// Three things are deliberately not in the list, and they are written
// down because a list that claims to name everything cannot leave the
// reader guessing whether an absence is a decision or an oversight.
// Item 64 was exactly that: the courier was missing from here, and
// nothing distinguished its absence from the two below.
//
// The fountain steps through three spray frames a second apart, which a
// slow tick renders exactly. The lamps' night glow is fixed by the zoom
// and the hour, not by the clock between frames. And the usage gauges
// hold still on purpose once they reach their reading, so a gauge's
// position along the river *is* its reading. They move only between
// readings and on their way in and out (item 79), and that is listed.

// Animating reports whether anything is in motion: a tug on the river,
// a refresh courier crossing it, a usage boat driving to its reading, a train on the loop, a car on an
// avenue, a drone over a roof, a worker at a door, a beacon pulsing,
// smoke rising, or a merge being celebrated. With motion reduced the
// clock stands still and nothing is, whatever is on the map — except a
// road growing over, which is a change and not a motion, and becomes a
// short cross-fade rather than nothing (item 85).
func (s *Scene) Animating() bool {
	if s.overgrowing() || s.mowingAny() || s.settlingAny() || s.hauling() || s.building() || s.flying() || s.prMoving() {
		return true
	}
	if s.reduced {
		return false
	}
	c := s.city
	if c == nil {
		return false
	}
	if len(s.voyages) > 0 || len(c.Trains) > 0 {
		return true
	}
	// Asked of the clock rather than of a flag, and without pruning:
	// Couriers() does the pruning and allocates a copy for the renderer,
	// which is too much to do from a gate that runs every Update. The
	// rule is the same one either way — a courier is in flight if its
	// own progress says so, decided per read.
	for _, courier := range s.couriers {
		if courier.Progress(s.now()) < 1 {
			return true
		}
	}
	if s.gaugesMoving() {
		return true
	}
	for i := range c.Streets {
		if c.Streets[i].Road != nil && c.Streets[i].Road.Active > 0 && len(c.Streets[i].Path) > 1 {
			return true
		}
	}
	for _, b := range c.Buildings() {
		if b.Blinds || b.Vacant {
			continue
		}
		if b.Pulse || b.Cranes > 0 || b.Smoke > 0 || b.Session.State == state.Working {
			return true
		}
		if s.Celebration(b.Session.ID) >= 0 {
			return true
		}
	}
	return false
}
