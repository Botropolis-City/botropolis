package city

import "botropolis.city/botropolis/pkg/state"

// plantFade is how long the power plant's windows take to come on or go
// off, in seconds: a building's lights take about as long.
const plantFade = 1.0

// PlantLights is how far the power plant's windows are lit, from 0 to 1:
// on while any session is working (Aria, 2026-10-20, item 185).
func (s *Scene) PlantLights() float64 {
	if !s.plantFaded {
		return s.plantTarget()
	}
	return s.plantLights
}

// plantTarget is 1 while any session is working, and 0 otherwise.
func (s *Scene) plantTarget() float64 {
	if s.city == nil {
		return 0
	}
	for _, b := range s.city.Buildings() {
		if b.Session.State == state.Working {
			return 1
		}
	}
	return 0
}

// fadePlant moves the plant's lights toward the sessions, dt seconds on:
// at once the first time, or when motion is reduced.
func (s *Scene) fadePlant(dt float64) {
	target := s.plantTarget()
	if !s.plantFaded || s.reduced {
		s.plantLights, s.plantFaded = target, true
		return
	}
	step := dt / plantFade
	if s.plantLights < target {
		s.plantLights = min(target, s.plantLights+step)
	} else {
		s.plantLights = max(target, s.plantLights-step)
	}
}
