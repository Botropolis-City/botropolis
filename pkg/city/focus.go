package city

import "botropolis.city/botropolis/pkg/state"

// Snapshot is the snapshot the scene was last given.
func (s *Scene) Snapshot() state.Snapshot {
	return s.snapshot
}

// Groups are the snapshot's groups, most recently active first.
func (s *Scene) Groups() []state.Group {
	return s.snapshot.Groups
}

// FocusGroup focuses the map on a group: its members stay lit, the rest
// of the city recedes, and the camera frames the districts it runs
// through. It reports false when no building on the map is in the group
// (item 87).
func (s *Scene) FocusGroup(id string) bool {
	var bounds Rect
	found := false
	for _, b := range s.city.Buildings() {
		if b.Group == nil || b.Group.ID != id {
			continue
		}
		d := s.city.DistrictOf(b)
		if d == nil {
			continue
		}
		if !found {
			bounds = d.Rect
		} else {
			bounds = bounds.Union(d.Rect)
		}
		found = true
	}
	if !found {
		return false
	}
	s.focus, s.dive = id, 0
	s.touched = true
	s.camera.FitWithInsets(bounds, s.width, s.height, s.Insets())
	if s.camera.Zoom > FocusMaxZoom {
		s.camera.SetZoomAt(s.camera.WorldToScreen(bounds.Center()), FocusMaxZoom)
	}
	return true
}

// FocusMaxZoom is the closest a focus frames a group, so a group in one
// small district shows its streets rather than a wall.
const FocusMaxZoom = 1.0

// FocusedGroup is the group the map is focused on, or "".
func (s *Scene) FocusedGroup() string {
	return s.focus
}

func (s *Scene) ClearFocus() {
	s.focus, s.dive = "", 0
}
