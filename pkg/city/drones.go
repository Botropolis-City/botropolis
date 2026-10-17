package city

import (
	"strings"
	"time"
)

// A drone is one subagent in flight. Item 89.
//
// It takes off from a pad on its session's roof, flies to the district
// its subagent works in when that is another one, loiters there, and
// when the subagent finishes flies home and lands on its pad. At still
// it fades in and out where it loiters, with no climb and no flight.
//
// Like a tug's (bug 54a), a flight's ends are kept as whose roof and
// which district rather than as points, and resolved against the city
// on every read, so a re-plan cannot strand a drone over open ground.
const (
	TakeoffFor   = 800 * time.Millisecond
	LandFor      = 800 * time.Millisecond
	FlyFor       = 1500 * time.Millisecond
	DroneFadeFor = 300 * time.Millisecond
	// MaxDrones is the most drones one roof flies, as it always has.
	MaxDrones = 3
)

// FlightStage is what a drone is doing.
type FlightStage int

const (
	TakingOff FlightStage = iota
	Outbound
	Loitering
	Returning
	Landing
)

// Flight is one drone as it is now. On is the building it works on, or
// nil while it flies between districts; Over is the world point it is
// over, Climb how far it is from its pad to its working height, and Fade
// its opacity, which only a cross-fade lowers.
type Flight struct {
	Seat  int
	Stage FlightStage
	On    *Building
	Over  Point
	Climb float64
	Fade  float64
}

// flight is a drone's record: launched when its subagent started, ended
// when it finished, and the place the subagent works.
type flight struct {
	seat     int
	works    string
	launched time.Time
	ended    time.Time
	// left is where it was when it ended, how far up, and whose building
	// it was working on, by session.
	left      Point
	leftClimb float64
	leftOn    string
}

// noteDrones launches a drone for every new subagent in flight and ends
// one for every subagent that finished.
//
// The first snapshot launches nothing: a city opened onto subagents
// already at work finds their drones already on station.
func (s *Scene) noteDrones(now time.Time) {
	launched := now
	if s.known == nil {
		launched = now.Add(-TakeoffFor - FlyFor - DroneFadeFor)
	}
	seen := map[string]bool{}
	for _, b := range s.city.Buildings() {
		id := b.Session.ID
		seen[id] = true
		want := min(b.Session.SubagentsInFlight, MaxDrones)
		var live []*flight
		for _, f := range s.drones[id] {
			if f.ended.IsZero() {
				live = append(live, f)
			}
		}
		for i := len(live); i < want; i++ {
			works := ""
			if i < len(b.Session.SubagentDirs) {
				works = b.Session.SubagentDirs[i]
			}
			if s.drones == nil {
				s.drones = map[string][]*flight{}
			}
			s.drones[id] = append(s.drones[id], &flight{seat: s.freeSeat(id), works: works, launched: launched})
		}
		for i := len(live) - 1; i >= want; i-- {
			s.endFlight(id, live[i], now)
		}
	}
	for id, flights := range s.drones {
		if seen[id] {
			continue
		}
		for _, f := range flights {
			if f.ended.IsZero() {
				s.endFlight(id, f, now)
			}
		}
	}
}

func (s *Scene) freeSeat(id string) int {
	taken := map[int]bool{}
	for _, f := range s.drones[id] {
		taken[f.seat] = true
	}
	seat := 0
	for taken[seat] {
		seat++
	}
	return seat
}

// endFlight ends a drone where it is: one still taking off begins
// landing at once.
func (s *Scene) endFlight(id string, f *flight, now time.Time) {
	at := s.flightAt(id, f, now)
	f.ended, f.left, f.leftClimb = now, at.Over, at.Climb
	if at.On != nil {
		f.leftOn = at.On.Session.ID
	}
}

// Flights is every drone flying for a session, landed ones pruned.
func (s *Scene) Flights(id string) []Flight {
	now := s.now()
	var out []Flight
	kept := s.drones[id][:0]
	for _, f := range s.drones[id] {
		at := s.flightAt(id, f, now)
		if at.Fade <= 0 && !f.ended.IsZero() {
			continue
		}
		kept = append(kept, f)
		out = append(out, at)
	}
	if len(kept) == 0 {
		delete(s.drones, id)
	} else {
		s.drones[id] = kept
	}
	return out
}

// flying reports whether any drone is taking off, flying, landing or
// fading: a change, which plays at every level.
func (s *Scene) flying() bool {
	now := s.now()
	for id, flights := range s.drones {
		for _, f := range flights {
			at := s.flightAt(id, f, now)
			if at.Fade > 0 && (at.Stage != Loitering || at.Fade < 1 || !f.ended.IsZero()) {
				return true
			}
		}
	}
	return false
}

// returnFor is how long a drone takes home from where it ended.
func (s *Scene) returnFor(id string, f *flight) time.Duration {
	home := s.roof(id)
	if f.left == home {
		return 0
	}
	return FlyFor
}

// flightAt is a drone at now.
func (s *Scene) flightAt(id string, f *flight, now time.Time) Flight {
	mine, on := s.buildingOf(id), s.worksOn(id, f)
	home, away := s.roof(id), centre(on)
	out := Flight{Seat: f.seat, On: on, Over: away, Climb: 1, Fade: 1}
	if s.reduced {
		out.Stage = Loitering
		out.Fade = min(1, float64(now.Sub(f.launched))/float64(DroneFadeFor))
		if !f.ended.IsZero() {
			out.On, out.Over = s.buildingOf(f.leftOn), f.left
			out.Fade = max(0, 1-float64(now.Sub(f.ended))/float64(DroneFadeFor))
		}
		return out
	}
	if !f.ended.IsZero() {
		since := now.Sub(f.ended)
		back := s.returnFor(id, f)
		if since < back {
			p := float64(since) / float64(back)
			out.Stage, out.On, out.Over, out.Climb = Returning, nil, lerp(f.left, home, p), f.leftClimb
			return out
		}
		p := float64(since-back) / float64(LandFor)
		out.Stage, out.On, out.Over, out.Climb = Landing, mine, home, f.leftClimb*(1-min(1, p))
		if p >= 1 {
			out.Fade = 0
		}
		return out
	}
	since := now.Sub(f.launched)
	switch {
	case since < TakeoffFor:
		out.Stage, out.On, out.Over, out.Climb = TakingOff, mine, home, float64(since)/float64(TakeoffFor)
	case on != mine && since < TakeoffFor+FlyFor:
		out.Stage, out.On, out.Over = Outbound, nil, lerp(home, away, float64(since-TakeoffFor)/float64(FlyFor))
	default:
		out.Stage = Loitering
	}
	return out
}

func lerp(a, b Point, p float64) Point {
	p = min(1, max(0, p))
	return Point{X: a.X + (b.X-a.X)*p, Y: a.Y + (b.Y-a.Y)*p}
}

// buildingOf is a session's building, or nil.
func (s *Scene) buildingOf(id string) *Building {
	for _, b := range s.city.Buildings() {
		if b.Session.ID == id {
			return b
		}
	}
	return nil
}

// roof is the middle of a session's building, where its pad is.
func (s *Scene) roof(id string) Point { return centre(s.buildingOf(id)) }

func centre(b *Building) Point {
	if b == nil {
		return Point{}
	}
	return b.Rect.Center()
}

// worksOn is the building a drone works on: in the district its
// subagent works in when that is another, the building there that was
// active last; otherwise its own.
func (s *Scene) worksOn(id string, f *flight) *Building {
	mine := s.buildingOf(id)
	d := s.city.districtAt(f.works)
	if d == nil {
		return mine
	}
	var last *Building
	for _, b := range d.Buildings {
		if b.Session.ID == id {
			return mine
		}
		if b.Vacant {
			continue
		}
		if last == nil || b.Session.LastActivity.After(last.Session.LastActivity) {
			last = b
		}
	}
	if last == nil {
		return mine
	}
	return last
}

// districtAt is the district a path lies in: the one with the longest
// root containing it.
func (c *City) districtAt(path string) *District {
	if path == "" {
		return nil
	}
	path = ProjectRoot(path)
	var best *District
	for _, d := range c.Districts {
		if d.Depot || d.Root == "" {
			continue
		}
		if path != d.Root && !strings.HasPrefix(path, d.Root+"/") {
			continue
		}
		if best == nil || len(d.Root) > len(best.Root) {
			best = d
		}
	}
	return best
}
