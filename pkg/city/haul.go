package city

import "time"

// A haul is a truck carrying a packed-up session's crate from its lot to
// the depot, or back to its old lot when it is unpacked: the departure is
// drawn, as every departure is. Under reduced motion it is a short
// cross-fade. Item 86, slice 3.
type HaulKind int

const (
	HaulPack HaulKind = iota
	HaulUnpack
)

const (
	PackFor     = 2 * time.Second
	PackFadeFor = 300 * time.Millisecond
)

type Haul struct {
	SessionID string
	Kind      HaulKind
	From, To  Point
	Since     time.Time
	For       time.Duration
	Hue       int
}

// Progress is how far the haul has got at a time, from 0 to 1.
func (h Haul) Progress(now time.Time) float64 {
	return min(1, max(0, float64(now.Sub(h.Since))/float64(h.For)))
}

// At is where the truck is at a time, eased out on arrival.
func (h Haul) At(now time.Time) Point {
	p := h.Progress(now)
	p = 1 - (1-p)*(1-p)
	return Point{X: h.From.X + (h.To.X-h.From.X)*p, Y: h.From.Y + (h.To.Y-h.From.Y)*p}
}

type spot struct {
	at     Point
	packed bool
}

// noteHauls compares where each session stood at the last snapshot with
// where it stands now, and starts a haul for each that was packed up or
// unpacked between them.
func (s *Scene) noteHauls(now time.Time) {
	spots := map[string]spot{}
	for _, b := range s.city.Buildings() {
		id := b.Session.ID
		spots[id] = spot{at: b.Rect.Center(), packed: b.Packed}
		was, known := s.spots[id]
		if !known || was.packed == b.Packed {
			continue
		}
		kind := HaulPack
		if !b.Packed {
			kind = HaulUnpack
		}
		span := PackFor
		if s.reduced {
			span = PackFadeFor
		}
		s.hauls = append(s.hauls, Haul{SessionID: id, Kind: kind, From: was.at, To: b.Rect.Center(), Since: now, For: span, Hue: projectHue(ProjectRoot(b.Session.CWD))})
	}
	s.spots = spots
}

// Hauls is every truck still on its way.
func (s *Scene) Hauls() []Haul {
	now := s.now()
	kept := s.hauls[:0]
	for _, h := range s.hauls {
		if now.Sub(h.Since) < h.For {
			kept = append(kept, h)
		}
	}
	s.hauls = kept
	return append([]Haul(nil), kept...)
}

// Hauling is whether a session's crate, or its building, is still on a
// truck, so the map draws it on the truck rather than where it is going.
func (s *Scene) Hauling(id string) bool {
	now := s.now()
	for _, h := range s.hauls {
		if h.SessionID == id && now.Sub(h.Since) < h.For {
			return true
		}
	}
	return false
}

func (s *Scene) hauling() bool {
	now := s.now()
	for _, h := range s.hauls {
		if now.Sub(h.Since) < h.For {
			return true
		}
	}
	return false
}

// HaulProgress is how far a session's haul has got, from 0 to 1, or 1
// when it is on no truck: under reduced motion the map fades a session in
// at its destination by it.
func (s *Scene) HaulProgress(id string) float64 {
	now := s.now()
	for _, h := range s.hauls {
		if h.SessionID == id && now.Sub(h.Since) < h.For {
			return h.Progress(now)
		}
	}
	return 1
}
