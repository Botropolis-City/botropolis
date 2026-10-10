package city

import (
	"math"
	"time"

	"botropolis.city/botropolis/pkg/state"
)

// The worker's day. A session's rover leaves the door when a tool call
// starts and comes back when it finishes, so every trip you watch is a
// tool call you could have counted. Between calls it waits at the door,
// and with motion reduced it never leaves it. Decided 2026-09-18.

// TripFor is how long the worker takes to drive its route, door to kerb.
const TripFor = 2500 * time.Millisecond

// Trip is one worker's leg: which way it is going, when the leg began,
// and how far along the route it had got by then, so a call that ends
// early turns the rover round where it stands instead of teleporting it.
type Trip struct {
	Out       bool
	StartedAt time.Time
	From      float64
}

// Progress is how far along the route the worker is: 0 at the door, 1
// at the kerb.
func (t *Trip) Progress(now time.Time) float64 {
	moved := float64(now.Sub(t.StartedAt)) / float64(TripFor)
	if t.Out {
		return math.Min(1, t.From+moved)
	}
	return math.Max(0, t.From-moved)
}

// Route is the worker's way out of its building: the door, the service
// lane beside it, and the kerb where the block meets the avenue. Every
// point is inside the district's own block, so a rover never drives on
// an avenue and never drives through the building in front of it.
func (d *District) Route(b *Building) []Point {
	door := d.Door(b)
	if d == nil {
		return []Point{door}
	}
	lane := clampTo(b.Rect.Max.X+BuildingGap/2, d.Rect.Min.X+DoorInset, d.Rect.Max.X-DoorInset)
	kerb := math.Max(door.Y, d.Rect.Max.Y-DoorInset)
	return []Point{door, {X: lane, Y: door.Y}, {X: lane, Y: kerb}}
}

// Worker is where a session's rover stands now. With motion reduced it
// waits at the door.
func (s *Scene) Worker(b *Building) Point {
	route := s.city.DistrictOf(b).Route(b)
	trip := s.trips[b.Session.ID]
	if s.reduced || trip == nil {
		return route[0]
	}
	return alongPath(route, trip.Progress(s.now()))
}

// WorkerTurn is which way the worker faces: down the route on the way
// out, back up it on the way home, so a rover never drives backwards.
func (s *Scene) WorkerTurn(b *Building) int {
	if trip := s.trips[b.Session.ID]; trip != nil && !trip.Out && !s.reduced {
		return 270
	}
	return 90
}

// noteTrips compares each live session's tool call with the one it had
// in the last snapshot and turns its worker round: out when a call
// starts, back when it ends. The first snapshot only takes note, so the
// city does not open with every rover already on the road.
func (s *Scene) noteTrips(snapshot state.Snapshot, now time.Time) {
	if s.tools == nil {
		s.tools, s.trips = map[string]string{}, map[string]*Trip{}
	}
	here := map[string]bool{}
	for _, sess := range snapshot.Sessions {
		if !state.Live(sess.State) {
			continue
		}
		here[sess.ID] = true
		was, known := s.tools[sess.ID]
		s.tools[sess.ID] = sess.Tool
		if !known || (was != "") == (sess.Tool != "") {
			continue
		}
		from := 0.0
		if trip := s.trips[sess.ID]; trip != nil {
			from = trip.Progress(now)
		}
		s.trips[sess.ID] = &Trip{Out: sess.Tool != "", StartedAt: now, From: from}
	}
	for id := range s.tools {
		if !here[id] {
			delete(s.tools, id)
			delete(s.trips, id)
		}
	}
}

func hypot(a, b Point) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }

// alongPath is the point a fraction of the way down a polyline.
func alongPath(path []Point, p float64) Point {
	if len(path) == 0 {
		return Point{}
	}
	if len(path) == 1 {
		return path[0]
	}
	total := 0.0
	for i := 1; i < len(path); i++ {
		total += hypot(path[i], path[i-1])
	}
	if total == 0 {
		return path[0]
	}
	want := math.Min(1, math.Max(0, p)) * total
	for i := 1; i < len(path); i++ {
		leg := hypot(path[i], path[i-1])
		if want <= leg || i == len(path)-1 {
			at := 0.0
			if leg > 0 {
				at = math.Min(1, want/leg)
			}
			return Point{
				X: path[i-1].X + (path[i].X-path[i-1].X)*at,
				Y: path[i-1].Y + (path[i].Y-path[i-1].Y)*at,
			}
		}
		want -= leg
	}
	return path[len(path)-1]
}
