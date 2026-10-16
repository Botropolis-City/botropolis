package city

import (
	"slices"
	"time"
)

// SightingFor is how long a first sighting's caption stays (item 88).
const SightingFor = 6 * time.Second

// Sight is told which kinds are on screen. A kind seen for the first
// time on this machine is captioned once, for SightingFor, and
// remembered in the layout so it is never captioned again.
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
		if s.sightings == nil {
			s.sightings = map[Kind]time.Time{}
		}
		s.sightings[k] = s.now()
	}
}

// Sightings are the kinds whose first-sighting caption is up now.
func (s *Scene) Sightings() []Kind {
	var up []Kind
	for k, at := range s.sightings {
		if s.now().Sub(at) < SightingFor {
			up = append(up, k)
		}
	}
	slices.Sort(up)
	return up
}
