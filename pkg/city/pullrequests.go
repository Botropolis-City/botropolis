package city

import (
	"math"
	"time"

	"botropolis.city/botropolis/pkg/claude"
)

// Pull requests on the map. Item 89.
//
// A pull request opening hoists a flag on its session's roof, and an
// inspector's car parks outside for as long as the session has one
// open. Merging one loads a crate onto a delivery truck, which drives
// the streets to the bridge and off the island; the merge that leaves a
// group with nothing open cuts its ribbon, with small fireworks, once.
// A pull request closed without merging lowers its flag and nothing
// more. At still the flag stands or falls at once, and the car, the
// truck and the ribbon are short cross-fades.
const (
	FlagFor   = 800 * time.Millisecond
	ParkFor   = 1500 * time.Millisecond
	RibbonFor = 2500 * time.Millisecond
	PRFadeFor = 300 * time.Millisecond
	// TruckSpeed is how fast a delivery truck drives, in world units a
	// second.
	TruckSpeed = 3 * CellSize
)

// Bridge is the one way off the island by road: from a street at the
// coast out over the sea, on the side away from the river.
//
// Item 132: the rail stays on the ground and the road goes over it. The
// landing cell is the abutment; the road climbs from its centre over the
// ramp's cells to the first span, over the rail (item 146), and the deck
// stays up from there out to Far.
type Bridge struct {
	Land, Far Point
	Abutment  Cell
	// Ramp is every cell the road climbs over between the abutment
	// and the deck.
	Ramp []Cell
	// Deck is every cell the raised road stands over, from the one
	// after the abutment out to Far's.
	Deck []Cell
}

// RampRun is how far the road climbs over, from the landing's centre to
// where the first span starts.
func (b Bridge) RampRun() float64 {
	return CellSize/2 + float64(len(b.Ramp))*CellSize
}

// Bridge is where the bridge stands. It follows from the plan alone, so
// the same plan always has it in the same place.
func (c *City) Bridge() Bridge {
	land, ok := c.bridgeCell()
	if !ok {
		return Bridge{}
	}
	at := land.Center()
	reach := c.Island().Reach()
	far := Point{X: reach.Min.X - CellSize, Y: at.Y}
	step := -1
	if c.bridgeEast() {
		far.X = reach.Max.X + CellSize
		step = 1
	}
	rail := map[Cell]bool{}
	for _, p := range c.Rails {
		rail[CellOf(p)] = true
	}
	b := Bridge{Land: at, Far: far, Abutment: land}
	for col := land.Col + step; col != CellOf(far).Col+step; col += step {
		cell := Cell{Col: col, Row: land.Row}
		if len(b.Deck) == 0 && !rail[cell] {
			b.Ramp = append(b.Ramp, cell)
			continue
		}
		b.Deck = append(b.Deck, cell)
	}
	return b
}

// Rise is how far up the bridge a point on its road is, from 0 on the
// ground to 1 on the deck: the road climbs from the landing's centre to
// the first span (item 146).
func (b Bridge) Rise(p Point) float64 {
	if b.Land == b.Far || math.Abs(p.Y-b.Land.Y) > CellSize/2 {
		return 0
	}
	out := p.X - b.Land.X
	if b.Far.X < b.Land.X {
		out = -out
	}
	return min(1, max(0, out/b.RampRun()))
}

// bridgeEast is whether the bridge leaves by the east coast: the side
// the river is not on.
func (c *City) bridgeEast() bool {
	from, _, _, ok := c.RiverBand()
	return ok && from.X < c.bounds.Center().X
}

// bridgeCell is the street cell nearest the coast on the bridge's side,
// the middlemost of those that reach it most directly.
func (c *City) bridgeCell() (Cell, bool) {
	lo, hi := CellOf(c.bounds.Min), CellOf(Point{X: c.bounds.Max.X - 1, Y: c.bounds.Max.Y - 1})
	mid := float64(lo.Row+hi.Row) / 2
	east := c.bridgeEast()
	var best Cell
	found := false
	better := func(a, b Cell) bool {
		if a.Col != b.Col {
			if east {
				return a.Col > b.Col
			}
			return a.Col < b.Col
		}
		return math.Abs(float64(a.Row)-mid) < math.Abs(float64(b.Row)-mid)
	}
	for row := lo.Row; row <= hi.Row; row++ {
		for i := 0; i <= hi.Col-lo.Col; i++ {
			col := lo.Col + i
			if east {
				col = hi.Col - i
			}
			cell := Cell{Col: col, Row: row}
			if !c.IsStreet(cell) {
				continue
			}
			if !found || better(cell, best) {
				best, found = cell, true
			}
			break
		}
	}
	return best, found
}

// IsStreet is whether a cell is one of the plan's avenues.
func (c *City) IsStreet(cell Cell) bool { return c.plan.IsStreet(fromCell(cell)) }

// toBridge is a delivery truck's way from a building's kerb along the
// avenues to the bridge and over it.
func (c *City) toBridge(b *Building) []Point {
	d := c.DistrictOf(b)
	bridge := c.Bridge()
	start := b.Rect.Center()
	if d != nil {
		route := d.Route(b)
		start = route[len(route)-1]
	}
	path := []Point{start}
	land, ok := c.bridgeCell()
	if dw, out := c.Driveway(d); ok && out {
		// Out of the lot by its driveway, as every road leaves it. Item 108.
		path = append(path, dw.Lot)
		for _, cell := range c.walkStreets(map[Cell]bool{dw.Cell: true}, map[Cell]bool{land: true}) {
			path = append(path, cell.Center())
		}
	}
	return append(path, bridge.Land, bridge.Far)
}

// InspectorRun is how far the inspector's car drives in along +x to
// park.
const InspectorRun = 3 * Tile

// Inspector is the inspector's car outside a building: how far it is
// into its parking space, whether it is leaving, where it is, and its
// opacity, which only a cross-fade lowers.
type Inspector struct {
	At Point
	// Park is where it stands once parked at the kerb.
	Park    Point
	Parked  float64
	Leaving bool
	Fade    float64
}

// Delivery is a crate leaving on a truck for a merged pull request.
type Delivery struct {
	SessionID string
	PR        claude.PR
	Path      []Point
	Since     time.Time
	// Fades is a delivery at still: the truck fades out at the kerb.
	Fades bool
}

// For is how long the delivery takes.
func (d Delivery) For() time.Duration {
	if d.Fades {
		return PRFadeFor
	}
	return time.Duration(pathLength(d.Path) / TruckSpeed * float64(time.Second))
}

// Progress is how far along it is, from 0 to 1.
func (d Delivery) Progress(now time.Time) float64 {
	span := d.For()
	if span <= 0 {
		return 1
	}
	return min(1, max(0, float64(now.Sub(d.Since))/float64(span)))
}

// At is where the truck is.
func (d Delivery) At(now time.Time) Point {
	if d.Fades {
		return d.Path[0]
	}
	return alongPath(d.Path, d.Progress(now))
}

// Ribbon is a group's ribbon being cut, over a building, P of the way
// through.
type Ribbon struct {
	Group     string
	At        Point
	P         float64
	Fireworks bool
}

type hoist struct {
	since time.Time
	down  bool
}

type parking struct {
	since   time.Time
	leaving bool
}

type ribbon struct {
	group string
	id    string
	since time.Time
}

// notePRs compares every pull request with the last snapshot's.
func (s *Scene) notePRs(now time.Time) {
	first := !s.opened
	seen := map[string]string{}
	open := map[string]bool{}
	groupOpen := map[string]bool{}
	for _, b := range s.city.Buildings() {
		for _, pr := range b.Session.PRs {
			if pr.State == "" || pr.State == claude.PROpen {
				open[b.Session.ID] = true
				groupOpen[groupOf(b)] = true
			}
		}
	}
	for group := range groupOpen {
		delete(s.celebrated, group)
	}
	for _, b := range s.city.Buildings() {
		id := b.Session.ID
		for _, pr := range b.Session.PRs {
			key := id + " " + pr.URL
			state := pr.State
			if state == "" {
				state = claude.PROpen
			}
			seen[key] = state
			was, known := s.prs[key]
			if first || (known && was == state) {
				continue
			}
			switch {
			case state == claude.PROpen:
				s.setHoist(key, hoist{since: now})
			case state == claude.PRClosed && was == claude.PROpen:
				s.setHoist(key, hoist{since: now, down: true})
			case state == claude.PRMerged:
				s.deliveries = append(s.deliveries, Delivery{SessionID: id, PR: pr, Path: s.city.toBridge(b), Since: now, Fades: s.reduced})
				if group := groupOf(b); !groupOpen[group] && !s.celebrated[group] {
					if s.celebrated == nil {
						s.celebrated = map[string]bool{}
					}
					s.celebrated[group] = true
					s.ribbons = append(s.ribbons, ribbon{group: group, id: id, since: now})
				}
			}
		}
	}
	s.prs = seen
	for _, b := range s.city.Buildings() {
		id := b.Session.ID
		p, parked := s.parked[id]
		switch {
		case open[id] && (!parked || p.leaving):
			since := now
			if first {
				since = now.Add(-ParkFor)
			}
			s.setParking(id, parking{since: since})
		case !open[id] && parked && !p.leaving:
			s.setParking(id, parking{since: now, leaving: true})
		}
	}
}

func groupOf(b *Building) string {
	if b.Session.Group != "" {
		return "group " + b.Session.Group
	}
	return "session " + b.Session.ID
}

func (s *Scene) setHoist(key string, h hoist) {
	if s.hoists == nil {
		s.hoists = map[string]hoist{}
	}
	s.hoists[key] = h
}

func (s *Scene) setParking(id string, p parking) {
	if s.parked == nil {
		s.parked = map[string]parking{}
	}
	s.parked[id] = p
}

// Hoist is how far up a pull request's flag is, from 0 to 1.
func (s *Scene) Hoist(id string, number int) float64 {
	b := s.buildingOf(id)
	if b == nil {
		return 1
	}
	for _, pr := range b.Session.PRs {
		if pr.Number != number {
			continue
		}
		h, ok := s.hoists[id+" "+pr.URL]
		if !ok {
			break
		}
		if s.reduced {
			if h.down {
				return 0
			}
			return 1
		}
		p := min(1, max(0, float64(s.now().Sub(h.since))/float64(FlagFor)))
		if h.down {
			return 1 - p
		}
		return p
	}
	return 1
}

// Inspector is the inspector's car outside a session's building, if one
// is there.
func (s *Scene) Inspector(id string) (Inspector, bool) {
	p, ok := s.parked[id]
	b := s.buildingOf(id)
	if !ok || b == nil {
		return Inspector{}, false
	}
	span := ParkFor
	if s.reduced {
		span = PRFadeFor
	}
	t := min(1, max(0, float64(s.now().Sub(p.since))/float64(span)))
	if p.leaving && t >= 1 {
		delete(s.parked, id)
		return Inspector{}, false
	}
	out := Inspector{Leaving: p.leaving, Parked: t, Fade: 1}
	if p.leaving {
		out.Parked = 1 - t
	}
	if s.reduced {
		out.Fade, out.Parked = out.Parked, 1
	}
	kerb := b.Rect.Center()
	if d := s.city.DistrictOf(b); d != nil {
		route := d.Route(b)
		kerb = route[len(route)-1]
	}
	out.Park = Point{X: kerb.X - Tile*0.6, Y: kerb.Y}
	out.At = Point{X: out.Park.X - (1-out.Parked)*InspectorRun, Y: kerb.Y}
	return out, true
}

// Deliveries is every truck still on its way off the island.
func (s *Scene) Deliveries() []Delivery {
	now := s.now()
	kept := s.deliveries[:0]
	for _, d := range s.deliveries {
		if d.Progress(now) < 1 {
			kept = append(kept, d)
		}
	}
	s.deliveries = kept
	return append([]Delivery(nil), kept...)
}

// Ribbons is every ribbon being cut.
func (s *Scene) Ribbons() []Ribbon {
	now := s.now()
	span := RibbonFor
	if s.reduced {
		span = PRFadeFor
	}
	var out []Ribbon
	kept := s.ribbons[:0]
	for _, r := range s.ribbons {
		p := float64(now.Sub(r.since)) / float64(span)
		if p >= 1 {
			continue
		}
		kept = append(kept, r)
		out = append(out, Ribbon{Group: r.group, At: centre(s.buildingOf(r.id)), P: max(0, p), Fireworks: !s.reduced})
	}
	s.ribbons = kept
	return out
}

func pathLength(path []Point) float64 {
	total := 0.0
	for i := 1; i < len(path); i++ {
		total += math.Hypot(path[i].X-path[i-1].X, path[i].Y-path[i-1].Y)
	}
	return total
}

// prMoving is whether a flag, a car, a truck or a ribbon is moving.
func (s *Scene) prMoving() bool {
	now := s.now()
	for _, h := range s.hoists {
		if !s.reduced && now.Sub(h.since) < FlagFor {
			return true
		}
	}
	for id := range s.parked {
		if in, ok := s.Inspector(id); ok && (in.Parked < 1 || in.Fade < 1 || in.Leaving) {
			return true
		}
	}
	return len(s.Deliveries()) > 0 || len(s.Ribbons()) > 0
}
