package city

import "botropolis.city/botropolis/pkg/events"

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

// SetRightChrome reserves screen room down the right, for the stream,
// so a fit keeps the city beside it and never under it.
func (s *Scene) SetRightChrome(px float64) {
	s.reserve(&s.right, px, Point{})
}
