package city

import "time"

// A haul is a truck carrying a packed-up session's crate from its lot to
// the depot, or back to its old lot when it is unpacked: the departure is
// drawn, as every departure is. Under reduced motion it is a short
// cross-fade. Item 86, slice 3. A move carries a building from one lot to
// another when Aria changes its session's group (item 348).
type HaulKind int

const (
	HaulPack HaulKind = iota
	HaulUnpack
	HaulMove
)

const (
	PackMin     = 2 * time.Second
	PackMax     = 6 * time.Second
	PackFadeFor = 300 * time.Millisecond
	// ConvoyGap is how long after one truck the next of a convoy
	// leaves, and ConvoyMax the latest the last of one leaves, so a
	// sweep of hundreds is still gone in seconds (item 333).
	ConvoyGap = 400 * time.Millisecond
	ConvoyMax = 4 * time.Second
)

// convoyGap is the gap between trucks when n leave together.
func convoyGap(n int) time.Duration {
	if n < 2 {
		return 0
	}
	return min(ConvoyGap, ConvoyMax/time.Duration(n-1))
}

// HaulFor is how long a haul from one place to another takes: driven at
// the city's truck speed, as deliveries are, so a far depot takes
// longer, but never under PackMin or over PackMax; a cross-fade under
// reduced motion (Aria, 2026-10-27, item 304).
func HaulFor(from, to Point, reduced bool) time.Duration {
	if reduced {
		return PackFadeFor
	}
	driven := time.Duration(pathLength([]Point{from, to}) / TruckSpeed * float64(time.Second))
	return min(PackMax, max(PackMin, driven))
}

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

// Fades is how much of a moving building shows at a time: at its old
// lot, fading out over the move's first PackFadeFor, and at its new one,
// fading in over its last, while the truck carries its crate between
// (Aria, 2026-11-06, item 348). A haul that is not a move fades neither.
func (h Haul) Fades(now time.Time) (old, new float64) {
	if h.Kind != HaulMove {
		return 0, 1
	}
	since := now.Sub(h.Since)
	fade := min(PackFadeFor, h.For)
	old = 1 - min(1, max(0, float64(since)/float64(fade)))
	new = min(1, max(0, float64(since-(h.For-fade))/float64(fade)))
	return old, new
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
// unpacked between them, and a move for each whose lot the layout
// changed by hand. Several leaving together go as a convoy, one after
// another; under reduced motion they cross-fade at once.
func (s *Scene) noteHauls(now time.Time) {
	first := len(s.hauls)
	defer func() { s.convoy(first, now) }()
	moved := map[string]bool{}
	for _, id := range s.city.Moved {
		moved[id] = true
	}
	spots := map[string]spot{}
	for _, b := range s.city.Buildings() {
		id := b.Session.ID
		spots[id] = spot{at: b.Rect.Center(), packed: b.Packed}
		was, known := s.spots[id]
		if !known {
			continue
		}
		if was.packed == b.Packed {
			if moved[id] && !b.Packed && was.at != b.Rect.Center() {
				span := HaulFor(was.at, b.Rect.Center(), s.reduced)
				s.hauls = append(s.hauls, Haul{SessionID: id, Kind: HaulMove, From: was.at, To: b.Rect.Center(), Since: now, For: span, Hue: projectHue(SessionRoot(b.Session))})
			}
			continue
		}
		kind := HaulPack
		if !b.Packed {
			kind = HaulUnpack
		}
		span := HaulFor(was.at, b.Rect.Center(), s.reduced)
		s.hauls = append(s.hauls, Haul{SessionID: id, Kind: kind, From: was.at, To: b.Rect.Center(), Since: now, For: span, Hue: projectHue(SessionRoot(b.Session))})
	}
	s.spots = spots
}

// convoy staggers the pack hauls, and the moves, started from index
// first on, each kind as a convoy of its own.
func (s *Scene) convoy(first int, now time.Time) {
	if s.reduced {
		return
	}
	s.stagger(first, HaulPack, now)
	s.stagger(first, HaulMove, now)
}

func (s *Scene) stagger(first int, kind HaulKind, now time.Time) {
	var packs []int
	for i := first; i < len(s.hauls); i++ {
		if s.hauls[i].Kind == kind {
			packs = append(packs, i)
		}
	}
	gap := convoyGap(len(packs))
	for n, i := range packs {
		s.hauls[i].Since = now.Add(time.Duration(n) * gap)
	}
}

// retargetHauls sends every truck on its way to where its building now
// stands. A haul's end is laid when it starts, and a re-layout within
// its span can move the building. The truck keeps its place, its
// progress and its arrival: only the start it is eased from is solved
// again, so it does not jump or speed up. Item 224.
func (s *Scene) retargetHauls(now time.Time) {
	at := map[string]*Building{}
	for _, b := range s.city.Buildings() {
		at[b.Session.ID] = b
	}
	for i, h := range s.hauls {
		b := at[h.SessionID]
		p := h.Progress(now)
		if b == nil || b.Packed != (h.Kind == HaulPack) || p >= 1 || b.Rect.Center() == h.To {
			continue
		}
		here, to := h.At(now), b.Rect.Center()
		eased := 1 - (1-p)*(1-p)
		s.hauls[i].From = here.Sub(to.Scale(eased)).Scale(1 / (1 - eased))
		s.hauls[i].To = to
	}
}

// Hauls is every truck still on its way.
func (s *Scene) Hauls() []Haul {
	now := s.now()
	var kept []Haul
	for _, h := range s.hauls {
		if now.Sub(h.Since) < h.For {
			kept = append(kept, h)
		}
	}
	return kept
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
// at its destination by it. A moving building fades in at its new lot by
// its move's fade (item 348).
func (s *Scene) HaulProgress(id string) float64 {
	now := s.now()
	for _, h := range s.hauls {
		if h.SessionID == id && now.Sub(h.Since) < h.For {
			if h.Kind == HaulMove {
				_, in := h.Fades(now)
				return in
			}
			return h.Progress(now)
		}
	}
	return 1
}

// Riding is whether a session's crate is on a truck for packing or
// unpacking, so the map draws it on the truck and not on a lot. A moving
// building is not riding: it fades out at one lot and in at the other
// (item 348).
func (s *Scene) Riding(id string) bool {
	now := s.now()
	for _, h := range s.hauls {
		if h.SessionID == id && h.Kind != HaulMove && now.Sub(h.Since) < h.For {
			return true
		}
	}
	return false
}

// unpacking is whether a truck is still leaving the depot with a crate.
func (s *Scene) unpacking() bool {
	now := s.now()
	for _, h := range s.hauls {
		if h.Kind == HaulUnpack && now.Sub(h.Since) < h.For {
			return true
		}
	}
	return false
}

func (c *City) depot() *District {
	for _, d := range c.Districts {
		if d.Depot {
			return d
		}
	}
	return nil
}

// holdDepot keeps the depot on the map, empty, while the truck carrying
// its last crate is still leaving it, so the truck does not drive away
// from bare grass.
func (s *Scene) holdDepot() {
	if s.city.depot() != nil || !s.unpacking() {
		return
	}
	s.keepDepot = true
	s.city = s.adopt(build(s.snapshot, s.layout, s.ladder, true, s.ledger, s.memo, s.offMap))
	s.city.Face(s.camera.Heading)
}

// releaseDepot lets an empty depot go once its truck has left.
func (s *Scene) releaseDepot() {
	if !s.keepDepot || s.unpacking() {
		return
	}
	s.keepDepot = false
	s.city = s.adopt(build(s.snapshot, s.layout, s.ladder, false, s.ledger, s.memo, s.offMap))
	s.city.Face(s.camera.Heading)
	s.generation++
	s.carryOn(s.now())
	s.noteGround()
}
