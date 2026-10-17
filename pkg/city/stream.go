package city

import (
	"math"

	"botropolis.city/botropolis/pkg/events"
)

// The stream is the event log's live tail, docked as a column beside
// the map (item 88). It stays calm by construction: routine entries are
// muted, and only what needs you or went wrong stands out.

// Flagged is whether an event stands out in the stream: needs-you and
// errors. Everything else is routine and muted.
func Flagged(k events.Kind) bool {
	return k == events.NeedsYou || k == events.Error
}

// FlyToEvent selects the event's session and centres the camera on it,
// and reports false when the session is no longer on the map.
func (s *Scene) FlyToEvent(e events.Event) bool {
	return s.Select(e.SessionID)
}

// SetCornerChrome reserves a panel in a right-hand corner, for the
// stream (item 100), so a fit keeps the city clear of it; an empty rect
// takes it away.
func (s *Scene) SetCornerChrome(panel Rect) {
	if panel == s.corner {
		return
	}
	s.corner = panel
	if !s.touched {
		s.fit()
	}
}

// cornerInsets reserves the corner panel as a strip down the right or
// along its top or bottom edge, whichever leaves the city larger.
func (s *Scene) cornerInsets(in Insets) Insets {
	if s.corner.Area() == 0 {
		return in
	}
	side := in
	side.Right = math.Max(in.Right, s.width-s.corner.Min.X)
	edge := in
	if s.corner.Center().Y < s.height/2 {
		edge.Top = math.Max(in.Top, s.corner.Max.Y)
	} else {
		edge.Bottom = math.Max(in.Bottom, s.height-s.corner.Min.Y)
	}
	if s.fitScale(edge) > s.fitScale(side) {
		return edge
	}
	return side
}

// fitScale is how large a fit within the insets would draw the city.
func (s *Scene) fitScale(in Insets) float64 {
	b := s.camera.Bounds(s.city.Extent())
	if b.Width() <= 0 || b.Height() <= 0 {
		return 0
	}
	return math.Min((s.width-in.Left-in.Right)/b.Width(), (s.height-in.Top-in.Bottom)/b.Height())
}
