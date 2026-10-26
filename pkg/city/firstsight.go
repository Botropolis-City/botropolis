package city

import "time"

const (
	// SightingFor is how long a first sighting's caption stays, and
	// SightingGap the pause before the next new kind's (item 88).
	SightingFor = 6 * time.Second
	SightingGap = 2 * time.Second
)

// Sight is told which kinds are on screen. A kind seen for the first
// time on this machine is captioned once, for SightingFor, and
// remembered in the layout so it is never captioned again. Several new
// at once are captioned one at a time, SightingGap apart, so a first
// launch does not caption everything at once.
func (s *Scene) Sight(visible []Kind) {
	if s.layout == nil {
		return
	}
	for _, k := range visible {
		name := k.Name()
		if name == "" || s.layout.Sighted[name] {
			continue
		}
		if s.layout.Sighted == nil {
			s.layout.Sighted = map[string]bool{}
		}
		s.layout.Sighted[name] = true
		if len(s.sightings) == 0 {
			s.sightingAt = s.now()
		}
		s.sightings = append(s.sightings, k)
	}
}

// Sightings are the kinds whose first-sighting caption is up now: one
// at most.
func (s *Scene) Sightings() []Kind {
	step := SightingFor + SightingGap
	for len(s.sightings) > 0 && s.now().Sub(s.sightingAt) >= step {
		s.sightings = s.sightings[1:]
		s.sightingAt = s.sightingAt.Add(step)
	}
	if len(s.sightings) == 0 || s.now().Sub(s.sightingAt) >= SightingFor {
		return nil
	}
	return s.sightings[:1:1]
}

// MarkSighted records kinds as captioned already, without captioning
// them, so a frame test's city draws what an everyday city does: no
// first sighting, whose caption comes down on a timer (item 260).
func (s *Scene) MarkSighted(kinds []Kind) {
	if s.layout == nil {
		return
	}
	if s.layout.Sighted == nil {
		s.layout.Sighted = map[string]bool{}
	}
	for _, k := range kinds {
		if name := k.Name(); name != "" {
			s.layout.Sighted[name] = true
		}
	}
}
