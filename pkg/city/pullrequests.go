package city

import (
	"math"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/plan"
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
	// Haze is where the deck starts to fade into the distance, out
	// over open sea; it is gone by Far (item 219).
	Haze     Point
	Abutment Cell
	// Ramp is every cell the road climbs over between the abutment
	// and the deck.
	Ramp []Cell
	// Deck is every cell the raised road stands over, from the one
	// after the abutment out to Far's.
	Deck []Cell
}

// HazeCells is how far the deck runs on into the haze, in cells, from
// where it starts to fade to where it is gone. Item 219.
const HazeCells = 3.0

// Alpha is how much of the bridge shows at a point on its road: all of
// it up to the haze, and none by the far end.
func (b Bridge) Alpha(p Point) float64 {
	band := math.Abs(b.Far.X - b.Haze.X)
	if b.Land == b.Far || band == 0 {
		return 1
	}
	into := (p.X - b.Haze.X) * math.Copysign(1, b.Far.X-b.Land.X)
	return min(1, max(0, 1-into/band))
}

// RampRun is how far the road climbs over, from the landing's centre to
// where the first span starts.
func (b Bridge) RampRun() float64 {
	return CellSize/2 + float64(len(b.Ramp))*CellSize
}

// SecondBridgeAt is how many districts the island holds when a second
// bridge leaves by the far coast (Aria, 2026-10-25). Item 220.
const SecondBridgeAt = 16

// landing is a street cell a bridge lands on, and whether the bridge
// leaves from it eastward.
type landing struct {
	cell Cell
	east bool
}

// Bridge is where the first bridge stands. It follows from the plan
// alone, so the same plan always has it in the same place.
func (c *City) Bridge() Bridge {
	if len(c.landings) == 0 {
		return Bridge{}
	}
	return c.bridgeFrom(c.landings[0])
}

// Bridges is every bridge off the island, the first first: one, and a
// second on the far coast once the island has held SecondBridgeAt
// districts. Item 220.
func (c *City) Bridges() []Bridge {
	var out []Bridge
	for _, l := range c.landings {
		out = append(out, c.bridgeFrom(l))
	}
	return out
}

// BridgeOf is the bridge a truck's path ends on, or the first if it ends
// on none of them.
func (c *City) BridgeOf(path []Point) Bridge {
	bridges := c.Bridges()
	for _, b := range bridges {
		if len(path) > 0 && path[len(path)-1] == b.Far {
			return b
		}
	}
	return c.Bridge()
}

func (c *City) bridgeFrom(l landing) Bridge {
	land := l.cell
	at := land.Center()
	reach := c.Island().Reach()
	haze := Point{X: reach.Min.X - CellSize, Y: at.Y}
	step := -1
	if l.east {
		haze.X = reach.Max.X + CellSize
		step = 1
	}
	far := Point{X: haze.X + float64(step)*HazeCells*CellSize, Y: at.Y}
	rail := map[Cell]bool{}
	for _, p := range c.Rails {
		rail[CellOf(p)] = true
	}
	b := Bridge{Land: at, Far: far, Haze: haze, Abutment: land}
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

// bridgeCell is where the first bridge lands, as placeBridge placed it.
func (c *City) bridgeCell() (Cell, bool) {
	if len(c.landings) == 0 {
		return Cell{}, false
	}
	return c.landings[0].cell, true
}

// projectDistricts is how many districts the island holds, the depot
// aside.
func (c *City) projectDistricts() int {
	n := 0
	for _, d := range c.Districts {
		if !d.Depot {
			n++
		}
	}
	return n
}

// placeBridge lands the bridge on the street cell nearest the coast on
// its side, among those that reach it most directly: on the row the
// layout remembers while that row is one of them, else the one nearest
// it, and for a layout that remembers none the middlemost. The row is
// remembered by its place among the plan's slot rows, so the bridge
// stays beside the same blocks as the island grows, and across a
// restart (Aria, 2026-10-24: "It should stay where it is"). Item 218.
//
// The second bridge is placed the same way on the far coast, once the
// island holds SecondBridgeAt districts, and kept from then on by its
// own row in the layout, whatever the count does after. Item 220.
func (c *City) placeBridge() {
	c.landings = nil
	east := c.bridgeEast()
	var kept, kept2 *plan.RowMark
	if c.layout != nil {
		kept, kept2 = c.layout.Bridge, c.layout.Bridge2
	}
	cell, ok := c.coastalStreet(kept, east)
	if !ok {
		return
	}
	c.landings = append(c.landings, landing{cell: cell, east: east})
	c.remember(&kept, cell)
	if kept2 == nil && c.projectDistricts() < SecondBridgeAt {
		c.keepMarks(kept, nil)
		return
	}
	if cell, ok := c.coastalStreet(kept2, !east); ok {
		c.landings = append(c.landings, landing{cell: cell, east: !east})
		c.remember(&kept2, cell)
	}
	c.keepMarks(kept, kept2)
}

// remember sets a mark to a landing's row.
func (c *City) remember(mark **plan.RowMark, cell Cell) {
	m := c.plan.MarkRow(fromCell(cell).Row)
	*mark = &m
}

// keepMarks writes the landings' rows into the layout, if there is one.
func (c *City) keepMarks(first, second *plan.RowMark) {
	if c.layout == nil {
		return
	}
	c.layout.Bridge = first
	if second != nil {
		c.layout.Bridge2 = second
	}
}

// coastalStreet is the street cell nearest the coast on one side, of
// those that reach it most directly: the one nearest the kept row, or
// with none kept the middlemost.
func (c *City) coastalStreet(kept *plan.RowMark, east bool) (Cell, bool) {
	lo, hi := CellOf(c.bounds.Min), CellOf(Point{X: c.bounds.Max.X - 1, Y: c.bounds.Max.Y - 1})
	mid := float64(lo.Row+hi.Row) / 2
	target := mid
	if kept != nil {
		target = float64(c.plan.RowOf(*kept))
	}
	var best Cell
	found := false
	better := func(a, b Cell) bool {
		if a.Col != b.Col {
			if east {
				return a.Col > b.Col
			}
			return a.Col < b.Col
		}
		if da, db := math.Abs(float64(a.Row)-target), math.Abs(float64(b.Row)-target); da != db {
			return da < db
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
// avenues to the nearer bridge by road, and over it (item 220).
func (c *City) toBridge(b *Building) []Point {
	var best []Point
	for _, bridge := range c.Bridges() {
		path := c.toBridgeBy(b, bridge)
		if best == nil || pathLength(path[:len(path)-1]) < pathLength(best[:len(best)-1]) {
			best = path
		}
	}
	if best == nil {
		return c.toBridgeBy(b, Bridge{})
	}
	return best
}

func (c *City) toBridgeBy(b *Building, bridge Bridge) []Point {
	d := c.DistrictOf(b)
	start := b.Rect.Center()
	if d != nil {
		route := d.Route(b)
		start = route[len(route)-1]
	}
	path := []Point{start}
	land, ok := CellOf(bridge.Land), bridge.Land != bridge.Far
	if dw, out := c.Driveway(d); ok && out {
		// Out of the lot by its driveway, as every road leaves it. Item 108.
		path = append(path, dw.Lot)
		for _, cell := range c.walkRoads(map[Cell]bool{dw.Cell: true}, map[Cell]bool{land: true}) {
			path = append(path, cell.Center())
		}
	}
	return append(path, bridge.Land, bridge.Far)
}

// rerouteDeliveries sends every truck on its way by the bridge as it now
// stands. A truck's path is laid at the merge, and the bridge stands on
// the middlemost street at the coast, so a city that grows a row while
// the truck drives moves the bridge out from under it. Item 217.
//
// With two bridges, a truck keeps to the bridge on the coast it was
// heading for (item 220).
func (s *Scene) rerouteDeliveries(now time.Time) {
	bridges := s.city.Bridges()
	if len(bridges) == 0 || bridges[0].Land == bridges[0].Far {
		return
	}
	for i, d := range s.deliveries {
		n := len(d.Path)
		if d.Fades || n < 2 || standsOn(d.Path, bridges) {
			continue
		}
		bridge := bridges[0]
		east := d.Path[n-1].X > d.Path[n-2].X
		for _, b := range bridges {
			if (b.Far.X > b.Land.X) == east {
				bridge = b
			}
		}
		s.deliveries[i].Path = s.city.reroute(d.Path, d.Progress(now), bridge)
		s.deliveries[i].Since = now
	}
}

// standsOn is whether a path ends over one of the bridges as it stands.
func standsOn(path []Point, bridges []Bridge) bool {
	n := len(path)
	for _, b := range bridges {
		if path[n-2] == b.Land && path[n-1] == b.Far {
			return true
		}
	}
	return false
}

// reroute is the rest of a truck's way from p of the way along path,
// whose last two points are the bridge it was laid for: along the deck
// as far out as it was, or on to the first avenue ahead and from there
// by the avenues to the landing.
func (c *City) reroute(path []Point, p float64, bridge Bridge) []Point {
	was := Bridge{Land: path[len(path)-2], Far: path[len(path)-1]}
	here := alongPath(path, p)
	if out := (here.X - was.Land.X) * math.Copysign(1, was.Far.X-was.Land.X); out > 0 && math.Abs(here.Y-was.Land.Y) <= CellSize/2 {
		dir := math.Copysign(1, bridge.Far.X-bridge.Land.X)
		return []Point{{X: bridge.Land.X + dir*out, Y: bridge.Land.Y}, bridge.Far}
	}
	land := CellOf(bridge.Land)
	route := []Point{here}
	for _, pt := range ahead(path[:len(path)-2], p*pathLength(path)) {
		if !c.IsStreet(CellOf(pt)) {
			route = append(route, pt)
			continue
		}
		for _, cell := range c.walkRoads(map[Cell]bool{CellOf(pt): true}, map[Cell]bool{land: true}) {
			route = append(route, cell.Center())
		}
		break
	}
	return append(route, bridge.Land, bridge.Far)
}

// ahead is every point of path further along than dist.
func ahead(path []Point, dist float64) []Point {
	walked := 0.0
	for i := 1; i < len(path); i++ {
		walked += math.Hypot(path[i].X-path[i-1].X, path[i].Y-path[i-1].Y)
		if walked > dist {
			return path[i:]
		}
	}
	return nil
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
