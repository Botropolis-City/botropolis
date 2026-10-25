package render

import (
	"hash/fnv"
	"math"
	"strconv"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

const kitSapling = "nature-kit/tree_small_dark"

// overgrowthPieces are what grows on a lot quiet for long enough: grass
// and weeds, and now and then a sapling (item 86).
var overgrowthPieces = []string{
	"nature-kit/grass", "nature-kit/grass_large", "nature-kit/grass_leafs", "nature-kit/plant_flatShort", kitSapling,
}

// weed is one piece of growth and where on the lot it stands.
type weed struct {
	Piece string
	At    city.Point
}

// lot is what a building and the ground around it carry for its rung.
// The building shows it in its windows: a beacon and a sign while it is
// live, dark glass once it stops, planks once it is shuttered. The
// ground ages: its paving wears, cracks once shuttered, and grows grass
// in the cracks, at the base and in the yard once overgrown, or while
// the growth is being mowed.
type lot struct {
	Beacon, Sign, Dark, Boards bool
	Wear                       float64
	Cracks                     [][]city.Point
	Weeds                      []weed
}

// wear is how stained a lot's paving is on each rung.
var wear = map[state.Rung]float64{
	state.RungResting:   0.2,
	state.RungShuttered: 0.42,
	state.RungOvergrown: 0.6,
}

// dressLot dresses a building's lot. mowing is how far a mowing has got,
// from 0 to 1, or below 0 when there is none: the growth clears in the
// order it was scattered.
func dressLot(b *city.Building, mowing float64) lot {
	d := lot{Beacon: !b.Blinds, Sign: !b.Blinds, Dark: b.Blinds, Boards: b.BoardedUp, Wear: wear[b.Rung]}
	if b.BoardedUp {
		d.Cracks = cracks(b)
	}
	if !b.Overgrown && mowing < 0 {
		return d
	}
	weeds := growth(b, cracks(b))
	if mowing > 0 {
		weeds = weeds[min(len(weeds), int(math.Round(mowing*float64(len(weeds))))):]
	}
	d.Weeds = weeds
	return d
}

// frontReach is how much further out than behind growth reaches in
// front of a building, as a share of its half of the gap.
const frontReach = 1.6

// lotRect is a building's lot: its footprint and its half of the gap to
// its neighbours, reaching further out in front, where it is seen.
func lotRect(b *city.Building) city.Rect {
	g := city.BuildingGap / 2
	return city.Rect{Min: city.Point{X: b.Rect.Min.X - g, Y: b.Rect.Min.Y - g}, Max: city.Point{X: b.Rect.Max.X + frontReach*g, Y: b.Rect.Max.Y + frontReach*g}}
}

func shrink(r city.Rect, by float64) city.Rect {
	dx, dy := r.Width()*by, r.Height()*by
	return city.Rect{Min: city.Point{X: r.Min.X + dx, Y: r.Min.Y + dy}, Max: city.Point{X: r.Max.X - dx, Y: r.Max.Y - dy}}
}

// yard is a point in the ring of ground around the footprint: side 0 to
// 3 (north, east, south, west), along it from 0 to 1, and out from the
// wall from 0 at its base to 1 at the lot's edge.
func yard(b *city.Building, side int, along, out float64) city.Point {
	r, g := b.Rect, city.BuildingGap/2*out
	switch side % 4 {
	case 0:
		return city.Point{X: r.Min.X + along*r.Width(), Y: r.Min.Y - g}
	case 1:
		return city.Point{X: r.Max.X + g, Y: r.Min.Y + along*r.Height()}
	case 2:
		return city.Point{X: r.Min.X + along*r.Width(), Y: r.Max.Y + g}
	}
	return city.Point{X: r.Min.X - g, Y: r.Min.Y + along*r.Height()}
}

// draw is the i-th seeded number for a lot, from 0 to 1.
func draw(b *city.Building, what string, i int) float64 {
	f := fnv.New32a()
	_, _ = f.Write([]byte(b.Session.ID + "/" + what + "/" + strconv.Itoa(i)))
	return float64(f.Sum32()%10000) / 10000
}

// cracks are the lot's cracked paving: short broken lines running along
// the ground beside the walls.
func cracks(b *city.Building) [][]city.Point {
	n := 3 + int(draw(b, "cracks", 0)*3)
	out := make([][]city.Point, 0, n)
	for i := 0; i < n; i++ {
		side := int(draw(b, "side", i) * 4)
		along := 0.1 + 0.6*draw(b, "along", i)
		var crack []city.Point
		for step := 0; step < 4; step++ {
			from := 0.15 + 0.7*draw(b, "jitter", i*4+step)
			crack = append(crack, yard(b, side, along+0.07*float64(step), from))
		}
		out = append(out, crack)
	}
	return out
}

// frontTufts is how many tufts grow along each front edge, one in each
// stretch of it, so the growth runs the whole length of the front.
const frontTufts = 7

// growth is what grows on an overgrown lot: grass in each crack, tufts
// the whole length of both front edges reaching out into the yard, a few
// at the back, and now and then a sapling, seeded by the session's id so
// a lot always grows the same way.
func growth(b *city.Building, cracks [][]city.Point) []weed {
	grass := overgrowthPieces[:len(overgrowthPieces)-1]
	var out []weed
	for i, c := range cracks {
		out = append(out, weed{Piece: grass[int(draw(b, "crackgrass", i)*float64(len(grass)))], At: c[1]})
	}
	for _, side := range []int{1, 2} {
		for i := 0; i < frontTufts; i++ {
			n := side*frontTufts + i
			along := (float64(i) + 0.2 + 0.6*draw(b, "frontalong", n)) / frontTufts
			at := yard(b, side, along, 0.1+(frontReach-0.2)*draw(b, "frontout", n))
			out = append(out, weed{Piece: grass[int(draw(b, "front", n)*float64(len(grass)))], At: at})
		}
	}
	tufts := 4 + int(draw(b, "tufts", 0)*3)
	for i := 0; i < tufts; i++ {
		side := 3 * int(draw(b, "tuftside", i)*2)
		at := yard(b, side, 0.05+0.9*draw(b, "tuftalong", i), 0.05+0.85*draw(b, "tuftout", i))
		out = append(out, weed{Piece: grass[int(draw(b, "tuft", i)*float64(len(grass)))], At: at})
	}
	if draw(b, "sapling", 0) < 0.6 {
		side := 1 + int(draw(b, "saplingside", 0)*2)
		out = append(out, weed{Piece: kitSapling, At: yard(b, side, 0.2+0.6*draw(b, "saplingalong", 0), 0.6)})
	}
	return out
}

// districtGround is a whole district's floor once every session on it
// has reached a rung: it wears, cracks once shuttered, and once overgrown
// turns mossy and grows edge to edge (item 86).
type districtGround struct {
	Wear   float64
	Mossy  bool
	Cracks [][]city.Point
	Weeds  []weed
}

// districtStep is how far apart growth stands on an overgrown district's
// floor.
const districtStep = city.Tile * 1.1

// dressDistrict dresses a district's floor for its rung: nothing while
// any session on it is live.
func dressDistrict(d *city.District) districtGround {
	if d.Depot || d.Rung == "" || d.Rung == state.RungLive {
		return districtGround{}
	}
	g := districtGround{Wear: wear[d.Rung], Mossy: d.Rung == state.RungOvergrown}
	r := d.Rect
	pad := city.Point{X: r.Width() * 0.02, Y: r.Height() * 0.02}
	onFloor := func(p city.Point) bool {
		if p.X < r.Min.X+pad.X || p.X > r.Max.X-pad.X || p.Y < r.Min.Y+pad.Y || p.Y > r.Max.Y-pad.Y {
			return false
		}
		for _, b := range d.Buildings {
			if b.Rect.Contains(p) {
				return false
			}
		}
		return true
	}
	seed := func(what string, i int) float64 {
		f := fnv.New32a()
		_, _ = f.Write([]byte(d.Root + "/" + what + "/" + strconv.Itoa(i)))
		return float64(f.Sum32()%10000) / 10000
	}
	if d.Rung == state.RungShuttered || d.Rung == state.RungOvergrown {
		n := 6 + int(r.Width()*r.Height()/(city.BuildingSize*city.BuildingSize))
		for i := 0; i < n; i++ {
			at := city.Point{X: r.Min.X + r.Width()*seed("cx", i), Y: r.Min.Y + r.Height()*seed("cy", i)}
			var crack []city.Point
			for step := 0; step < 4 && onFloor(at); step++ {
				crack = append(crack, at)
				at = at.Add(city.Point{X: (seed("dx", i*4+step) - 0.3) * city.Tile, Y: (seed("dy", i*4+step) - 0.5) * city.Tile * 0.6})
			}
			if len(crack) > 1 {
				g.Cracks = append(g.Cracks, crack)
			}
		}
	}
	if !g.Mossy {
		return g
	}
	grass := overgrowthPieces[:len(overgrowthPieces)-1]
	i := 0
	for y := r.Min.Y + districtStep/2; y < r.Max.Y; y += districtStep {
		for x := r.Min.X + districtStep/2; x < r.Max.X; x += districtStep {
			i++
			at := city.Point{X: x + (seed("jx", i)-0.5)*districtStep*0.8, Y: y + (seed("jy", i)-0.5)*districtStep*0.8}
			if !onFloor(at) {
				continue
			}
			piece := grass[int(seed("grass", i)*float64(len(grass)))]
			if seed("sapling", i) < 0.06 {
				piece = kitSapling
			}
			g.Weeds = append(g.Weeds, weed{Piece: piece, At: at})
		}
	}
	return g
}

// districtDresser keeps each district's floor while everything
// dressDistrict reads of it holds — its root, rung, depot, rect and
// its buildings' rects — so a snapshot's new but equal district is not
// dressed again (item 207). dressed counts the dressings it did.
type districtDresser struct {
	by      map[string]dressedDistrict
	dressed int
}

type dressedDistrict struct {
	rung   state.Rung
	depot  bool
	rect   city.Rect
	lots   []city.Rect
	ground districtGround
	// gen is the dressing this was, counted across every district.
	gen int
}

func (m *districtDresser) dress(d *city.District) districtGround {
	if m.by == nil {
		m.by = map[string]dressedDistrict{}
	}
	kept, ok := m.by[d.Root]
	if ok && kept.rung == d.Rung && kept.depot == d.Depot && kept.rect == d.Rect && sameLots(kept.lots, d.Buildings) {
		return kept.ground
	}
	m.dressed++
	kept = dressedDistrict{rung: d.Rung, depot: d.Depot, rect: d.Rect, lots: kept.lots[:0], ground: dressDistrict(d), gen: m.dressed}
	for _, b := range d.Buildings {
		kept.lots = append(kept.lots, b.Rect)
	}
	m.by[d.Root] = kept
	return kept.ground
}

// generation is which dressing a district's floor is, after dress has
// been asked for it: a number that moves whenever its floor does.
func (m *districtDresser) generation(d *city.District) int { return m.by[d.Root].gen }

func sameLots(lots []city.Rect, buildings []*city.Building) bool {
	if len(lots) != len(buildings) {
		return false
	}
	for i, b := range buildings {
		if lots[i] != b.Rect {
			return false
		}
	}
	return true
}
