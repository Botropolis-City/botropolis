package city

import (
	"math"
	"slices"
	"time"
)

// Traffic on the roads between districts. Items 89 and 255.
//
// The roads already count what passed between two districts, each way:
// messages, files edited across, and subagents' visits. Every unit one
// snapshot adds sends a vehicle of its kind along the road's street path
// from where it started to the other end: a van for a message, a box
// truck for a file, a motorbike for a visit. A group's line carries
// nothing. The first snapshot sends nothing, so opening the map does not
// replay history, and at still a vehicle fades in at its far end with no
// drive.

// VanGap is how far apart vehicles sent in one snapshot leave.
const VanGap = 400 * time.Millisecond

// Cargo is what a vehicle on a road carries.
type Cargo int

const (
	CargoMessage Cargo = iota
	CargoFiles
	CargoVisit
)

// Kind is the kind of vehicle that carries the load.
func (l Cargo) Kind() Kind {
	switch l {
	case CargoFiles:
		return KindBoxTruck
	case CargoVisit:
		return KindMotorbike
	}
	return KindVan
}

// Traffic is one vehicle on its way along a road.
type Traffic struct {
	Cargo    Cargo
	From, To *District
	Path     []Point
	Since    time.Time
	Fades    bool
}

// For is how long the trip takes.
func (m Traffic) For() time.Duration {
	if m.Fades {
		return PRFadeFor
	}
	return time.Duration(pathLength(m.Path) / TruckSpeed * float64(time.Second))
}

// Progress is how far along it is, from 0 to 1.
func (m Traffic) Progress(now time.Time) float64 {
	span := m.For()
	if span <= 0 {
		return 1
	}
	return min(1, max(0, float64(now.Sub(m.Since))/float64(span)))
}

// At is where the vehicle is.
func (m Traffic) At(now time.Time) Point {
	if m.Fades || len(m.Path) == 0 {
		if len(m.Path) == 0 {
			return Point{}
		}
		return m.Path[len(m.Path)-1]
	}
	return alongPath(m.Path, m.Progress(now))
}

var trafficCards = map[Cargo][2]string{
	CargoMessage: {"message", "one message between two sessions"},
	CargoFiles:   {"file edit", "one file a session edited in another project"},
	CargoVisit:   {"subagent", "one subagent working in another project"},
}

// TrafficCard is one vehicle: what it carries, from one district to
// another.
func TrafficCard(m Traffic) Card {
	from, to := "", ""
	if m.From != nil {
		from = m.From.Name
	}
	if m.To != nil {
		to = m.To.Name
	}
	text := trafficCards[m.Cargo]
	return Card{Title: text[0] + arrow + from + " → " + to, Lines: []string{text[1]}}
}

// roadCount is a road's traffic of each load, each way, at a snapshot.
type roadCount map[Cargo][2]int

func countsOf(r *RoadLine) roadCount {
	return roadCount{
		CargoMessage: {r.Messages, r.BackMessages},
		CargoFiles:   {r.Files, r.BackFiles},
		CargoVisit:   {r.Visits, r.BackVisits},
	}
}

// noteTraffic sends a vehicle for every unit of traffic each road gained.
func (s *Scene) noteTraffic(now time.Time) {
	first := !s.opened
	seen := map[string]roadCount{}
	for i := range s.city.Roads {
		road := &s.city.Roads[i]
		if road.From == nil || road.To == nil {
			continue
		}
		key := road.From.Root + "\x00" + road.To.Root
		counts := countsOf(road)
		seen[key] = counts
		if first {
			continue
		}
		was := s.counted[key]
		var out, back int
		for _, load := range []Cargo{CargoMessage, CargoFiles, CargoVisit} {
			out = s.send(load, road, road.From, road.To, counts[load][0]-was[load][0], out)
			back = s.send(load, road, road.To, road.From, counts[load][1]-was[load][1], back)
		}
	}
	s.counted = seen
}

// send sends n vehicles carrying load, queued behind the ones already
// leaving that way, and says how many are queued now.
func (s *Scene) send(load Cargo, road *RoadLine, from, to *District, n, queued int) int {
	if n <= 0 {
		return queued
	}
	path := s.roadPath(road, from)
	start := s.now()
	for k := 0; k < n; k++ {
		s.traffic = append(s.traffic, Traffic{Cargo: load, From: from, To: to, Path: path, Since: start.Add(time.Duration(queued+k) * VanGap), Fades: s.reduced})
	}
	return queued + n
}

// roadPath is the road's street path, turned to run from the given end.
func (s *Scene) roadPath(road *RoadLine, from *District) []Point {
	path := []Point{road.From.Rect.Center(), road.To.Rect.Center()}
	for _, st := range s.city.Streets {
		if st.Road == road {
			path = append([]Point(nil), st.Path...)
		}
	}
	if from != road.From {
		slices.Reverse(path)
	}
	return path
}

// rerouteTraffic sends every vehicle on its way by the road as it now
// stands, to its districts as they now stand. A vehicle's path is laid
// when it is sent, and a re-layout can move the road out from under it.
// Item 224, after item 217's trucks.
func (s *Scene) rerouteTraffic(now time.Time) {
	for i, m := range s.traffic {
		if m.From == nil || m.To == nil || m.Progress(now) >= 1 {
			continue
		}
		from, to := s.city.districtRooted(m.From.Root), s.city.districtRooted(m.To.Root)
		road := s.city.roadBetween(from, to)
		if road == nil {
			continue
		}
		want := s.roadPath(road, from)
		m.From, m.To = from, to
		switch rest := ahead(m.Path, m.Progress(now)*pathLength(m.Path)); {
		case m.Fades || !now.After(m.Since):
			m.Path = want
		case isTail(rest, want):
		default:
			m.Path, m.Since = s.city.rejoin(m.At(now), want), now
		}
		s.traffic[i] = m
	}
}

// rejoin is the way from here along the streets as they now stand to a
// street path: to the nearest street if here is off them, by the
// avenues to the path, and along it to its end.
func (c *City) rejoin(here Point, path []Point) []Point {
	end := path[len(path)-1]
	onto := map[Cell]bool{}
	for _, p := range path[1 : len(path)-1] {
		onto[CellOf(p)] = true
	}
	start := CellOf(here)
	if !c.IsStreet(start) {
		start = c.nearestStreet(here, end)
	}
	cells := c.walkStreets(map[Cell]bool{start: true}, onto)
	if len(cells) == 0 {
		return []Point{here, end}
	}
	if len(cells) > 1 && here.DistanceToSegment(cells[0].Center(), cells[1].Center()) < 1e-9 {
		cells = cells[1:]
	}
	route := []Point{here}
	for _, cell := range cells {
		route = append(route, cell.Center())
	}
	joined := cells[len(cells)-1]
	for j := 1; j < len(path)-1; j++ {
		if CellOf(path[j]) == joined {
			return append(route, path[j+1:]...)
		}
	}
	return append(route, end)
}

// nearestStreet is the avenue cell nearest a point, or of those about as
// near, the one nearest where it is going, so it does not back up.
func (c *City) nearestStreet(p, toward Point) Cell {
	nearest := math.Inf(1)
	for _, st := range c.plan.Streets {
		nearest = min(nearest, hypot(toCell(st.Cell).Center(), p))
	}
	var best Cell
	left := math.Inf(1)
	for _, st := range c.plan.Streets {
		cell := toCell(st.Cell)
		if hypot(cell.Center(), p) > nearest+BuildingSize/2 {
			continue
		}
		if d := hypot(cell.Center(), toward); d < left {
			best, left = cell, d
		}
	}
	return best
}

// isTail is whether rest is the last points of path.
func isTail(rest, path []Point) bool {
	return len(rest) <= len(path) && slices.Equal(rest, path[len(path)-len(rest):])
}

func (c *City) districtRooted(root string) *District {
	for _, d := range c.Districts {
		if d.Root == root {
			return d
		}
	}
	return nil
}

// roadBetween is the road joining two districts, either way, or nil.
func (c *City) roadBetween(a, b *District) *RoadLine {
	if a == nil || b == nil {
		return nil
	}
	for i := range c.Roads {
		r := &c.Roads[i]
		if (r.From == a && r.To == b) || (r.From == b && r.To == a) {
			return r
		}
	}
	return nil
}

// Traffic is every vehicle still on its way along a road.
func (s *Scene) Traffic() []Traffic {
	now := s.now()
	kept := s.traffic[:0]
	for _, m := range s.traffic {
		if m.Progress(now) < 1 {
			kept = append(kept, m)
		}
	}
	s.traffic = kept
	return append([]Traffic(nil), kept...)
}
