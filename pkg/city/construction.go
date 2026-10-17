package city

import "time"

// A new session's building is built where its tug docks. Item 89.
//
// The build plays once, in BuildFor: the lot is fenced, scaffolding and
// a crane go up, the floors rise to the building's full height, the
// scaffolding and crane come down and the lights come on. At still the
// same change is a short cross-fade with no site at all. A newer change
// to the same session — resting, waking, packing up — jumps the build to
// its end.
const (
	BuildFor     = 3 * time.Second
	BuildFadeFor = 300 * time.Millisecond
)

// Site is how far each part of a build has got, each from 0 to 1.
// Opacity is the building's own, which only a cross-fade lowers.
type Site struct {
	Fence, Scaffold, Crane, Floors, Lights, Opacity float64
}

// SiteAt is the build p of the way through, from 0 to 1.
func SiteAt(p float64) Site {
	up := ramp(p, 0.1, 0.25)
	down := 1 - ramp(p, 0.78, 0.88)
	return Site{
		Fence:    ramp(p, 0, 0.1) * down,
		Scaffold: up * down,
		Crane:    up * down,
		Floors:   ramp(p, 0.25, 0.75),
		Lights:   ramp(p, 0.88, 1),
		Opacity:  1,
	}
}

func ramp(p, from, to float64) float64 {
	return min(1, max(0, (p-from)/(to-from)))
}

func (s *Scene) buildFor() time.Duration {
	if s.reduced {
		return BuildFadeFor
	}
	return BuildFor
}

// Construction is the build on a session's lot, and whether one is
// under way: from its tug docking until the build is done.
func (s *Scene) Construction(id string) (Site, bool) {
	v := s.arriving(id)
	if v == nil {
		return Site{}, false
	}
	since := s.now().Sub(v.StartedAt) - SailFor
	if since < 0 {
		return Site{}, false
	}
	p := float64(since) / float64(s.buildFor())
	if p >= 1 {
		return Site{}, false
	}
	if s.reduced {
		return Site{Floors: 1, Lights: 1, Opacity: p}, true
	}
	return SiteAt(p), true
}

// building reports whether any lot has a build under way.
func (s *Scene) building() bool {
	for _, v := range s.voyages {
		if _, ok := s.Construction(v.SessionID); ok && v.Kind == Arrival {
			return true
		}
	}
	return false
}

// finishBuilds jumps to its end every build whose session has just begun
// another transition.
func (s *Scene) finishBuilds(now time.Time) {
	for _, v := range s.voyages {
		if v.Kind != Arrival {
			continue
		}
		t, settling := s.settling[v.SessionID]
		if (settling && t.since.Equal(now)) || s.Hauling(v.SessionID) {
			v.StartedAt = now.Add(-SailFor - s.buildFor())
		}
	}
}
