package city

import (
	"math"
	"slices"
	"time"
)

// Messages between sessions on the map. Item 89.
//
// The roads already count the messages passed between two districts,
// each way. Every message one snapshot adds sends a van along the road
// from the sender's district to the recipient's, or a carriage along
// the group's line when both are stations on a drawn one. The first
// snapshot sends nothing, so opening the map does not replay history,
// and at still a van fades in at the recipient's end with no drive.

// VanGap is how far apart vans for messages in one snapshot leave.
const VanGap = 400 * time.Millisecond

// Message is one message on its way: a van, or a carriage on a line.
type Message struct {
	From, To *District
	Path     []Point
	Since    time.Time
	Train    bool
	Fades    bool
}

// For is how long the trip takes.
func (m Message) For() time.Duration {
	if m.Fades {
		return PRFadeFor
	}
	return time.Duration(pathLength(m.Path) / TruckSpeed * float64(time.Second))
}

// Progress is how far along it is, from 0 to 1.
func (m Message) Progress(now time.Time) float64 {
	span := m.For()
	if span <= 0 {
		return 1
	}
	return min(1, max(0, float64(now.Sub(m.Since))/float64(span)))
}

// At is where the van is.
func (m Message) At(now time.Time) Point {
	if m.Fades || len(m.Path) == 0 {
		if len(m.Path) == 0 {
			return Point{}
		}
		return m.Path[len(m.Path)-1]
	}
	return alongPath(m.Path, m.Progress(now))
}

// MessageCard is one van: one message, from one district to another.
func MessageCard(m Message) Card {
	from, to := "", ""
	if m.From != nil {
		from = m.From.Name
	}
	if m.To != nil {
		to = m.To.Name
	}
	card := Card{Title: "message" + arrow + from + " → " + to, Lines: []string{"one message between two sessions"}}
	if m.Train {
		card.Lines = append(card.Lines, "along the group's line")
	}
	return card
}

type mailCount struct{ messages, back int }

// noteMessages sends a van for every message each road gained.
func (s *Scene) noteMessages(now time.Time) {
	first := !s.opened
	seen := map[string]mailCount{}
	for i := range s.city.Roads {
		road := &s.city.Roads[i]
		if road.From == nil || road.To == nil {
			continue
		}
		key := road.From.Root + "\x00" + road.To.Root
		now := mailCount{road.Messages, road.BackMessages}
		seen[key] = now
		if first {
			continue
		}
		was := s.mail[key]
		s.send(road, road.From, road.To, now.messages-was.messages)
		s.send(road, road.To, road.From, now.back-was.back)
	}
	s.mail = seen
}

func (s *Scene) send(road *RoadLine, from, to *District, n int) {
	if n <= 0 {
		return
	}
	path, train := s.messagePath(road, from, to)
	start := s.now()
	for k := 0; k < n; k++ {
		s.messages = append(s.messages, Message{From: from, To: to, Path: path, Since: start.Add(time.Duration(k) * VanGap), Train: train, Fades: s.reduced})
	}
}

// messagePath is the way a message goes: along the group's line between
// the two stations when one is drawn through both, and otherwise the
// road's street path, turned to run from the sender to the recipient.
func (s *Scene) messagePath(road *RoadLine, from, to *District) ([]Point, bool) {
	if !s.linesHidden {
		for _, line := range s.city.Lines {
			i, j := -1, -1
			for k, st := range line.Stations {
				if st.District == from {
					i = k
				}
				if st.District == to {
					j = k
				}
			}
			if i < 0 || j < 0 {
				continue
			}
			var path []Point
			for k := i; ; {
				path = append(path, line.Stations[k].At)
				if k == j {
					break
				}
				if j > i {
					k++
				} else {
					k--
				}
			}
			return path, true
		}
	}
	path := []Point{road.From.Rect.Center(), road.To.Rect.Center()}
	for _, st := range s.city.Streets {
		if st.Road == road {
			path = append([]Point(nil), st.Path...)
		}
	}
	if from != road.From {
		slices.Reverse(path)
	}
	return path, false
}

// rerouteMessages sends every van on its way by the road as it now
// stands, to its districts as they now stand. A van's path is laid when
// the message is sent, and a re-layout can move the road out from under
// it. Item 224, after item 217's trucks.
func (s *Scene) rerouteMessages(now time.Time) {
	for i, m := range s.messages {
		if m.From == nil || m.To == nil || m.Progress(now) >= 1 {
			continue
		}
		from, to := s.city.districtRooted(m.From.Root), s.city.districtRooted(m.To.Root)
		road := s.city.roadBetween(from, to)
		if road == nil {
			continue
		}
		want, train := s.messagePath(road, from, to)
		m.From, m.To = from, to
		switch rest := ahead(m.Path, m.Progress(now)*pathLength(m.Path)); {
		case m.Fades || !now.After(m.Since):
			m.Path, m.Train = want, train
		case isTail(rest, want) && m.Train == train:
		case train && m.Train && len(want) == len(m.Path):
			m.Path = append([]Point{m.At(now)}, want[len(want)-len(rest):]...)
			m.Since = now
		case train:
			m.Path, m.Since, m.Train = []Point{m.At(now), want[len(want)-1]}, now, true
		default:
			m.Path, m.Since, m.Train = s.city.rejoin(m.At(now), want), now, false
		}
		s.messages[i] = m
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

// Messages is every message still on its way.
func (s *Scene) Messages() []Message {
	now := s.now()
	kept := s.messages[:0]
	for _, m := range s.messages {
		if m.Progress(now) < 1 {
			kept = append(kept, m)
		}
	}
	s.messages = kept
	return append([]Message(nil), kept...)
}
