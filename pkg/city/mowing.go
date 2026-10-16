package city

import (
	"time"

	"botropolis.city/botropolis/pkg/state"
)

// An overgrown lot is mowed when its session wakes: the growth clears
// once, and under reduced motion the same change is a short cross-fade.
// Item 86.
const (
	MowFor     = 1500 * time.Millisecond
	MowFadeFor = 300 * time.Millisecond
)

// noteMowing starts a mowing on every lot whose session was overgrown at
// the last snapshot and is live at this one, and remembers each rung.
func (s *Scene) noteMowing(now time.Time) {
	rungs := map[string]state.Rung{}
	for _, b := range s.city.Buildings() {
		id := b.Session.ID
		rungs[id] = b.Rung
		if s.rungs[id] == state.RungOvergrown && b.Rung == state.RungLive {
			if s.mowing == nil {
				s.mowing = map[string]time.Time{}
			}
			s.mowing[id] = now
		}
	}
	s.rungs = rungs
}

// Mowing is how far a session's lot is through its mowing, from 0 to 1,
// or -1 when it is not being mowed.
func (s *Scene) Mowing(id string) float64 {
	since, ok := s.mowing[id]
	if !ok {
		return -1
	}
	p := float64(s.now().Sub(since)) / float64(s.mowFor())
	if p >= 1 {
		delete(s.mowing, id)
		return -1
	}
	return max(0, p)
}

func (s *Scene) mowFor() time.Duration {
	if s.reduced {
		return MowFadeFor
	}
	return MowFor
}

func (s *Scene) mowingAny() bool {
	now, span := s.now(), s.mowFor()
	for _, since := range s.mowing {
		if now.Sub(since) < span {
			return true
		}
	}
	return false
}
