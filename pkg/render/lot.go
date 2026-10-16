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

// lotRect is a building's lot: its footprint and its half of the gap to
// its neighbours.
func lotRect(b *city.Building) city.Rect {
	g := city.BuildingGap / 2
	return city.Rect{Min: city.Point{X: b.Rect.Min.X - g, Y: b.Rect.Min.Y - g}, Max: city.Point{X: b.Rect.Max.X + g, Y: b.Rect.Max.Y + g}}
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

// growth is what grows on an overgrown lot: grass in each crack, tufts
// at the base of the walls and in the yard, and now and then a sapling,
// seeded by the session's id so a lot always grows the same way.
func growth(b *city.Building, cracks [][]city.Point) []weed {
	grass := overgrowthPieces[:len(overgrowthPieces)-1]
	var out []weed
	for i, c := range cracks {
		out = append(out, weed{Piece: grass[int(draw(b, "crackgrass", i)*float64(len(grass)))], At: c[1]})
	}
	tufts := 10 + int(draw(b, "tufts", 0)*4)
	for i := 0; i < tufts; i++ {
		side := int(draw(b, "tuftside", i) * 4)
		at := yard(b, side, 0.05+0.9*draw(b, "tuftalong", i), 0.05+0.85*draw(b, "tuftout", i))
		out = append(out, weed{Piece: grass[int(draw(b, "tuft", i)*float64(len(grass)))], At: at})
	}
	if draw(b, "sapling", 0) < 0.6 {
		side := 1 + int(draw(b, "saplingside", 0)*2)
		out = append(out, weed{Piece: kitSapling, At: yard(b, side, 0.2+0.6*draw(b, "saplingalong", 0), 0.6)})
	}
	return out
}
