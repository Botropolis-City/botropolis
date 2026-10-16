package city

import (
	"path/filepath"
	"time"

	"botropolis.city/botropolis/pkg/state"
)

// The river carries the session lifecycle: a new session arrives on a
// tug from the north and docks beside its district before its building
// rises; a session that leaves the map goes downriver on one. Decided
// 2026-09-18.

type VoyageKind string

const (
	Arrival   VoyageKind = "arrival"
	Departure VoyageKind = "departure"
)

const (
	// SailFor is how long a tug takes from the river's end to the dock.
	SailFor = 6 * time.Second
	// RiseFor is how long a docked session's building takes to rise.
	RiseFor = 1500 * time.Millisecond
)

// Voyage is one tug on the river, and whose session it is carrying.
//
// From and To are NOT stored when the voyage begins. Bug 54a: the river
// sits at the map's east edge and the map's width follows the block
// layout, so the river moves whenever the city is re-planned — and it
// moves for far less than a new district. A second session in a project
// that already exists shifts it a whole cell, 48 units. A tug holding
// points captured at launch was then sailing a river that no longer
// existed, and when the map shrank those points lay past its east edge,
// where there is nothing to draw: Aria's "teleports off the river into
// the black".
//
// So the ends are parameters — which end of the river, and whose dock —
// and Voyages resolves them against the live city on every read. The
// invariant then holds at every read rather than at every write, which
// is the difference between a value that cannot drift and one that only
// stays right while every future writer remembers to update it.
type Voyage struct {
	SessionID string
	Title     string
	Project   string
	// Root is the project the dock belongs to, kept so the dock can be
	// found again after a re-layout.
	Root string
	Kind VoyageKind
	Hue  int
	// From and To are refreshed by Voyages from the city as it is now.
	// Reading them off a Voyage that did not come from there is reading
	// wherever the river happened to be last time.
	From, To Point
	// Handover is how far up the arrival run a departure begins, for a
	// tug that was still coming in when its session went away and was
	// turned round rather than deleted. Zero for one that starts at the
	// dock. A fraction rather than a point, for the same reason the
	// ends are: it has to survive the river moving.
	Handover  float64
	StartedAt time.Time
}

// Progress is how far along the river the tug is, 0 at From to 1 at To.
func (v *Voyage) Progress(now time.Time) float64 {
	p := float64(now.Sub(v.StartedAt)) / float64(SailFor)
	return min(1, max(0, p))
}

// At is where the tug is now.
func (v *Voyage) At(now time.Time) Point {
	p := v.Progress(now)
	return Point{X: v.From.X + (v.To.X-v.From.X)*p, Y: v.From.Y + (v.To.Y-v.From.Y)*p}
}

func (v *Voyage) Card() Card {
	verb := "arriving"
	if v.Kind == Departure {
		verb = "leaving"
	}
	return Card{Title: verb + ": " + v.Title, Lines: []string{
		"project  " + v.Project,
		"cargo    a " + ContainerHues[v.Hue] + " container",
	}}
}

// riverEnds is where the river enters and leaves the map, and its x.
// riverEnds are the river's mouths out at sea: arrivals come in at the
// north one and departures leave by the south. Item 85.
func (c *City) riverEnds() (north, south Point, ok bool) {
	if _, _, _, ok := c.RiverBand(); !ok {
		return Point{}, Point{}, false
	}
	north, south = c.Island().Mouths()
	return north, south, true
}

// dock is the river point beside a project's district, or beside the
// plaza when the project has no district on the map.
func (c *City) dock(root string) Point {
	north, _, _ := c.riverEnds()
	y := c.Plaza.Center().Y
	for _, d := range c.Districts {
		if d.Root == root && !d.Storage {
			y = d.Rect.Center().Y
		}
	}
	return Point{X: north.X, Y: y}
}

// noteVoyages compares the live sessions with the last snapshot's and
// launches a tug for each one that arrived or left; the first snapshot
// only takes note, so the city does not open with a fleet.
func (s *Scene) noteVoyages(snapshot state.Snapshot, now time.Time) {
	current := map[string]state.Session{}
	for _, sess := range snapshot.Sessions {
		if state.Live(sess.State) || sess.State == state.Empty {
			current[sess.ID] = sess
		}
	}
	first := s.known == nil
	// The river only has to exist for a voyage to be worth launching;
	// where its ends are is settled per frame by Voyages, not here.
	if _, _, ok := s.city.riverEnds(); !first && ok {
		for id, sess := range current {
			if _, was := s.known[id]; was {
				continue
			}
			root := ProjectRoot(sess.CWD)
			s.voyages = append(s.voyages, &Voyage{SessionID: id, Title: titleOrID(sess), Project: filepath.Base(root),
				Root: root, Kind: Arrival, Hue: projectHue(root), StartedAt: now})
		}
		for id, was := range s.known {
			if _, still := current[id]; still {
				continue
			}
			if _, parked := s.parkedNow(snapshot, id); parked {
				continue
			}
			// A tug still on its way in is turned round where it
			// stands rather than left to evaporate while a second one
			// is launched behind it. Aria: "it shouldn't just despawn,
			// it should run the whole length of the water". Bug 54.
			if v := s.arriving(id); v != nil {
				v.Kind, v.Handover, v.StartedAt = Departure, v.Progress(now), now
				continue
			}
			root := ProjectRoot(was.CWD)
			s.voyages = append(s.voyages, &Voyage{SessionID: id, Title: titleOrID(was), Project: filepath.Base(root),
				Root: root, Kind: Departure, Hue: projectHue(root), StartedAt: now})
		}
	}
	s.known = current
}

// arriving is the tug still bringing a session in, if there is one.
func (s *Scene) arriving(id string) *Voyage {
	for _, v := range s.voyages {
		if v.SessionID == id && v.Kind == Arrival {
			return v
		}
	}
	return nil
}

func (s *Scene) parkedNow(snapshot state.Snapshot, id string) (state.Session, bool) {
	for _, sess := range snapshot.Sessions {
		if sess.ID == id && sess.State == state.Parked {
			return sess, true
		}
	}
	return state.Session{}, false
}

func titleOrID(sess state.Session) string {
	if sess.Title != "" {
		return sess.Title
	}
	return state.ShortID(sess.ID)
}

// Voyages is every tug still on the river: arrivals until they dock
// and their building has risen, departures until they leave the map.
//
// Each one's ends are resolved here, against the city as it stands now,
// rather than remembered from when it launched. See Voyage. Progress is
// time-based, so re-anchoring keeps a tug's place along the run and
// simply moves the run under it.
func (s *Scene) Voyages() []*Voyage {
	now := s.now()
	kept := s.voyages[:0]
	for _, v := range s.voyages {
		if v.Kind == Departure && v.Progress(now) >= 1 {
			continue
		}
		if v.Kind == Arrival && now.Sub(v.StartedAt) >= SailFor+RiseFor {
			continue
		}
		kept = append(kept, v)
	}
	s.voyages = kept
	north, south, ok := s.city.riverEnds()
	if !ok {
		// No river, no boats. Better than sailing them across the land
		// on the last coordinates anyone happened to compute.
		return nil
	}
	for _, v := range s.voyages {
		dock := s.city.dock(v.Root)
		if v.Kind == Arrival {
			v.From, v.To = north, dock
			continue
		}
		from := dock
		if v.Handover > 0 {
			// Where the arrival had reached when it was turned round,
			// measured along today's river rather than yesterday's.
			from = Point{
				X: north.X + (dock.X-north.X)*v.Handover,
				Y: north.Y + (dock.Y-north.Y)*v.Handover,
			}
		}
		v.From, v.To = from, south
	}
	return append([]*Voyage(nil), s.voyages...)
}

// Rising is how far a session's building has risen: 0 while its tug is
// still at sea, climbing to 1 over RiseFor once it docks, and 1 for
// every building that did not arrive by river.
func (s *Scene) Rising(id string) float64 {
	now := s.now()
	for _, v := range s.voyages {
		if v.SessionID != id || v.Kind != Arrival {
			continue
		}
		since := now.Sub(v.StartedAt)
		if since < SailFor {
			return 0
		}
		return min(1, float64(since-SailFor)/float64(RiseFor))
	}
	return 1
}
