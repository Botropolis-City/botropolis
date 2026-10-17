package city

import (
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
	first := s.mail == nil
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
