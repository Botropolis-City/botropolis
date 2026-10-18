package render

import (
	"hash/fnv"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/plan"
	"botropolis.city/botropolis/pkg/state"
)

// Recipes: which kit piece stands for what. A session's building is as
// many storeys as its heft has reached on the user's own scale, in its
// district type's kind for that height (item 109).
var (
	// The city's own buildings, by district type and storeys: a kind
	// for each height, so floors can be counted (item 109).
	kinds = map[city.DistrictType][state.MaxStoreys]string{
		city.ResidentialDistrict: {"botropolis/residential-1", "botropolis/residential-2", "botropolis/residential-3", "botropolis/residential-4", "botropolis/residential-5"},
		city.ShoppingDistrict:    {"botropolis/shopping-1", "botropolis/shopping-2", "botropolis/shopping-3", "botropolis/shopping-4", "botropolis/shopping-5"},
		city.OfficeDistrict:      {"botropolis/office-1", "botropolis/office-2", "botropolis/office-3", "botropolis/office-4", "botropolis/office-5"},
		city.IndustrialDistrict:  {"botropolis/industrial-1", "botropolis/industrial-2", "botropolis/industrial-3", "botropolis/industrial-4", "botropolis/industrial-5"},
	}

	// The courses under each kind, one per compaction, the top one first
	// and each below it a step wider (item 109).
	kindCourses = map[string][maxCourses]string{
		"botropolis/residential-1": {"botropolis/residential-1-course-1", "botropolis/residential-1-course-2", "botropolis/residential-1-course-3"},
		"botropolis/residential-2": {"botropolis/residential-2-course-1", "botropolis/residential-2-course-2", "botropolis/residential-2-course-3"},
		"botropolis/residential-3": {"botropolis/residential-3-course-1", "botropolis/residential-3-course-2", "botropolis/residential-3-course-3"},
		"botropolis/residential-4": {"botropolis/residential-4-course-1", "botropolis/residential-4-course-2", "botropolis/residential-4-course-3"},
		"botropolis/residential-5": {"botropolis/residential-5-course-1", "botropolis/residential-5-course-2", "botropolis/residential-5-course-3"},
		"botropolis/shopping-1":    {"botropolis/shopping-1-course-1", "botropolis/shopping-1-course-2", "botropolis/shopping-1-course-3"},
		"botropolis/shopping-2":    {"botropolis/shopping-2-course-1", "botropolis/shopping-2-course-2", "botropolis/shopping-2-course-3"},
		"botropolis/shopping-3":    {"botropolis/shopping-3-course-1", "botropolis/shopping-3-course-2", "botropolis/shopping-3-course-3"},
		"botropolis/shopping-4":    {"botropolis/shopping-4-course-1", "botropolis/shopping-4-course-2", "botropolis/shopping-4-course-3"},
		"botropolis/shopping-5":    {"botropolis/shopping-5-course-1", "botropolis/shopping-5-course-2", "botropolis/shopping-5-course-3"},
		"botropolis/office-1":      {"botropolis/office-1-course-1", "botropolis/office-1-course-2", "botropolis/office-1-course-3"},
		"botropolis/office-2":      {"botropolis/office-2-course-1", "botropolis/office-2-course-2", "botropolis/office-2-course-3"},
		"botropolis/office-3":      {"botropolis/office-3-course-1", "botropolis/office-3-course-2", "botropolis/office-3-course-3"},
		"botropolis/office-4":      {"botropolis/office-4-course-1", "botropolis/office-4-course-2", "botropolis/office-4-course-3"},
		"botropolis/office-5":      {"botropolis/office-5-course-1", "botropolis/office-5-course-2", "botropolis/office-5-course-3"},
		"botropolis/industrial-1":  {"botropolis/industrial-1-course-1", "botropolis/industrial-1-course-2", "botropolis/industrial-1-course-3"},
		"botropolis/industrial-2":  {"botropolis/industrial-2-course-1", "botropolis/industrial-2-course-2", "botropolis/industrial-2-course-3"},
		"botropolis/industrial-3":  {"botropolis/industrial-3-course-1", "botropolis/industrial-3-course-2", "botropolis/industrial-3-course-3"},
		"botropolis/industrial-4":  {"botropolis/industrial-4-course-1", "botropolis/industrial-4-course-2", "botropolis/industrial-4-course-3"},
		"botropolis/industrial-5":  {"botropolis/industrial-5-course-1", "botropolis/industrial-5-course-2", "botropolis/industrial-5-course-3"},
	}

	// The skip at the kerb, by how full it is, and the trucks that come
	// for it and pour the course after (item 109).
	kitSkips           = [4]string{"botropolis/skip-1", "botropolis/skip-2", "botropolis/skip-3", "botropolis/skip-4"}
	kitSkipTruck       = "botropolis/skip-truck"
	kitSkipTruckLoaded = "botropolis/skip-truck-loaded"
	kitMixer           = "botropolis/mixer-truck"

	kitPlant = "city-kit-industrial/building-a"
	kitStack = "city-kit-industrial/chimney-medium"
	// Phase 21 item 38's signage prototypes. Only the gantry carries a
	// board; the other two are bare mounts.
	kitSignGantry = "city-kit-roads/sign-highway"
	kitSignPost   = "city-kit-roads/road-sign-empty"
	kitSignArm    = "city-kit-roads/road-sign-empty-hanging"
	kitHall       = "city-kit-commercial/building-n"
	kitLibrary    = "city-kit-commercial/building-l"
	kitTower      = "city-kit-industrial/water-tower"
	kitLamp       = "city-kit-roads/light-square"
	kitDrone      = "botropolis/drone"
	// The plaza's fountain, modelled in the pipeline like the drone:
	// three frames of spray, cycled slowly.
	kitFountain = []string{"botropolis/fountain-a", "botropolis/fountain-b", "botropolis/fountain-c"}
	// The traffic keeps to the cars no other mover drives: the delivery
	// truck and the inspector's SUV mean a pull request, and the van is
	// kept for a message (item 89).
	kitCars = []string{"car-kit/sedan", "car-kit/taxi", "car-kit/hatchback-sports"}
	// The park palette: the Suburban kit's two, and six of the Nature
	// Kit's cut to their height and repainted in their greens by the
	// pipeline. plan.ParkSpecies is how many the plan may ask for.
	kitParkTrees = []string{
		"city-kit-suburban/tree-small",
		"city-kit-suburban/tree-large",
		"nature-kit/tree_default",
		"nature-kit/tree_oak",
		"nature-kit/tree_thin",
		"nature-kit/tree_tall",
		"nature-kit/tree_pineRoundA",
		"nature-kit/tree_small",
	}
	// A street is planted in one of these, all the way down: the neat,
	// narrow ones.
	kitStreetTrees = []string{
		"city-kit-suburban/tree-small",
		"nature-kit/tree_thin",
		"nature-kit/tree_small",
	}
	// The plaza's edge.
	kitBushes  = []string{"nature-kit/plant_bush", "nature-kit/plant_bushLarge"}
	kitPlanter = "city-kit-suburban/planter"
	// The ledger's trains: a locomotive and container wagons in one of
	// three liveries, matched to city.ContainerHues.
	kitLocos = []string{"train-kit/train-diesel-a", "train-kit/train-diesel-b", "train-kit/train-diesel-c"}
	// The tugs on the river: one brings a session in, the other takes
	// one away.
	// The refresh courier. Item 56: it must not be mistaken for a tug,
	// which means something else entirely. Picked by measurement — the
	// tugs are 2.24 tall and blocky, every speed hull is 1.2 to 1.7 and
	// flat, and boat-speed-j is the longest and most slender of them at
	// 4.27 and 2.39:1 against boat-tug-a's 3.47 and 1.94:1. Low and
	// fast against tall and squat, on both axes.
	kitCourier = "watercraft-kit/boat-speed-j"

	kitTugIn  = "watercraft-kit/boat-tug-a"
	kitTugOut = "watercraft-kit/boat-tug-b"
	// The usage gauges' hull. Deliberately not a tug: the river already
	// means arrivals and departures, and two meanings on one waterway
	// need silhouettes that differ at a glance. Item 49.
	// One hull per gauge rank, so the three boats differ in profile
	// rather than only in scale. Item 51: long and tall, long and flat,
	// small and vertical. The sail is the one that earns its place —
	// it is the only hull taller than it is long, so the *smallest*
	// boat keeps the most legible silhouette at the zoom where
	// everything else collapses to a smudge. It also settles the tug
	// question: it is within a few tenths of a tug in length and cannot
	// be mistaken for one, because one of them has a sail.
	// Which way these face is set by eye against a frame, not derived:
	// see bowHome in gauges.go. The models do not determine a bow — the
	// liner tapers at both ends, and on the cargo ship the deckhouse and
	// the hull's taper point opposite ways. r254 shipped them backwards
	// on a reasoned guess, so do not re-derive this from the geometry.
	kitGaugeLiner = "watercraft-kit/ship-ocean-liner-small"
	kitGaugeCargo = "watercraft-kit/ship-cargo-a"
	kitGaugeSail  = "watercraft-kit/boat-sail-a"
	// The channel markers the boats are read against: a plain buoy at
	// halfway, a flagged one at the limit. Bug 50.
	kitBuoy      = "watercraft-kit/buoy"
	kitBuoyLimit = "watercraft-kit/buoy-flag"
	kitWagons    = []string{"train-kit/train-carriage-container-red", "train-kit/train-carriage-container-blue", "train-kit/train-carriage-container-green"}
)

func hashID(id string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return h.Sum32()
}

// crates are the shipping containers a packed-up session is kept in, in
// the order of city.ContainerHues (item 86).
var crates = []string{"city-kit-industrial/shipping-container-a", "city-kit-industrial/shipping-container-b", "city-kit-industrial/shipping-container-c"}

// buildingPiece is the piece for a session: its district type's kind
// for its storeys, or a crate in its district's colour once it is packed
// up. A stopped session keeps its building on its own lot (item 86).
func buildingPiece(b *city.Building) string {
	if b.Packed {
		return crates[b.Hue%len(crates)]
	}
	column, ok := kinds[b.Type]
	if !ok {
		column = kinds[city.ResidentialDistrict]
	}
	n := min(max(b.Session.Storeys, 1), state.MaxStoreys)
	return column[n-1]
}

// treePiece is the tree on a park cell: the suburban kit's two trees,
// which share the city kits' palette, the plan's seeded variant picking
// which so a block reads as a wood, not a hedge.
func treePiece(t city.Tree) string {
	switch t.Kind {
	case plan.StreetTree:
		return pickPiece(kitStreetTrees, t.Variant)
	case plan.Bush:
		return pickPiece(kitBushes, t.Variant)
	case plan.Planter:
		return kitPlanter
	default:
		return pickPiece(kitParkTrees, t.Variant)
	}
}

// pickPiece is the species a planting asked for, or the first when the
// plan offers more species than the palette has pieces.
func pickPiece(pieces []string, variant int) string {
	if variant < 0 || variant >= len(pieces) {
		return pieces[0]
	}
	return pieces[variant]
}

// roadPiece is the road piece and its turn for a street cell's joins.
//
// The kit's own geometry, measured off the atlas rather than assumed —
// each sprite read at all four baked headings, its open sides taken from
// the tile diamond's edge midpoints, where a closed side carries the
// raised kerb (near-white) and an open one is road surface (mid grey).
// At camera heading 0 the projection is screen = ((x-y)s, (x+y)s/2) with
// +X east and +Y south, so the diamond's upper-right edge faces north,
// lower-right east, lower-left south, upper-left west.
//
// At turn 0:
//   - a straight runs east–west
//   - a bend joins SOUTH and WEST, its arc bulging to the north-east
//   - a T has its bar east–west and its stem south (open E, S, W)
//   - an end is open to the east
//
// and every +90 of turn takes each open side one step along
// E -> N -> W -> S.
//
// The bend's line above used to read "joins north and west", which is a
// quarter turn out, and the table was built on it. Worse, it was built
// assuming turn rotates the other way round, which is invisible on the
// straight and the crossroad because both are symmetric under a half
// turn, and wrong on everything else — including the end piece, which
// nobody had noticed because a dead end is rare on a generated plan.
// Eight of the sixteen masks were wrong. TestRoadPieceOpensWhereItJoins
// holds all sixteen against the measured geometry, which is the check
// that would have caught it.
func roadPiece(mask int) (string, int) {
	n, e, s, w := mask&city.DirN != 0, mask&city.DirE != 0, mask&city.DirS != 0, mask&city.DirW != 0
	count := 0
	for _, v := range []bool{n, e, s, w} {
		if v {
			count++
		}
	}
	switch count {
	case 4:
		return "city-kit-roads/road-crossroad", 0
	case 3:
		// The stem is the side that is missing: south at turn 0, then
		// west, north, east as the turn comes round.
		switch {
		case !n:
			return "city-kit-roads/road-intersection", 0
		case !w:
			return "city-kit-roads/road-intersection", 90
		case !s:
			return "city-kit-roads/road-intersection", 180
		default:
			return "city-kit-roads/road-intersection", 270
		}
	case 2:
		switch {
		case e && w:
			return "city-kit-roads/road-straight", 0
		case n && s:
			return "city-kit-roads/road-straight", 90
		case s && w:
			return "city-kit-roads/road-bend", 0
		case s && e:
			return "city-kit-roads/road-bend", 90
		case n && e:
			return "city-kit-roads/road-bend", 180
		default:
			return "city-kit-roads/road-bend", 270
		}
	case 1:
		switch {
		case e:
			return "city-kit-roads/road-end", 0
		case n:
			return "city-kit-roads/road-end", 90
		case w:
			return "city-kit-roads/road-end", 180
		default:
			return "city-kit-roads/road-end", 270
		}
	}
	return "city-kit-roads/road-square", 0
}
