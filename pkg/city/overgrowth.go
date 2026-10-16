package city

import (
	"sort"
	"time"
)

// A road whose connection has ceased grows over: its cells keep their
// shape and fade into the belt once, and then they are belt. Under
// reduced motion the same change is a short cross-fade. Item 85.
const (
	OvergrowFor     = 1500 * time.Millisecond
	OvergrowFadeFor = 300 * time.Millisecond
)

// Overgrowth is one road cell growing over, and how far it has gone,
// from 0 for tarmac to 1 for belt.
type Overgrowth struct {
	Cell     Cell
	Mask     int
	Progress float64
}

type overgrowing struct {
	mask  int
	since time.Time
}

// noteOvergrowth starts every road cell the last city laid and this one
// does not growing over, and ends the transition on any cell a road has
// come back to.
func (s *Scene) noteOvergrowth(was []StreetCell, now time.Time) {
	road := map[Cell]bool{}
	for _, sc := range s.city.StreetCells {
		road[sc.Cell] = true
		delete(s.overgrowth, sc.Cell)
	}
	for _, sc := range was {
		if road[sc.Cell] {
			continue
		}
		if _, already := s.overgrowth[sc.Cell]; already {
			continue
		}
		if s.overgrowth == nil {
			s.overgrowth = map[Cell]overgrowing{}
		}
		s.overgrowth[sc.Cell] = overgrowing{mask: sc.Mask, since: now}
	}
}

func (s *Scene) overgrowFor() time.Duration {
	if s.reduced {
		return OvergrowFadeFor
	}
	return OvergrowFor
}

// Overgrowth is every road cell still growing over, in cell order.
func (s *Scene) Overgrowth() []Overgrowth {
	now, span := s.now(), s.overgrowFor()
	var out []Overgrowth
	for cell, g := range s.overgrowth {
		p := float64(now.Sub(g.since)) / float64(span)
		if p >= 1 {
			delete(s.overgrowth, cell)
			continue
		}
		out = append(out, Overgrowth{Cell: cell, Mask: g.mask, Progress: max(0, p)})
	}
	sort.Slice(out, func(i, j int) bool { return lessCell(out[i].Cell, out[j].Cell) })
	return out
}

func (s *Scene) overgrowing() bool {
	now, span := s.now(), s.overgrowFor()
	for _, g := range s.overgrowth {
		if now.Sub(g.since) < span {
			return true
		}
	}
	return false
}
