package city

import (
	"maps"
	"slices"

	"botropolis.city/botropolis/pkg/group"
	"botropolis.city/botropolis/pkg/plan"
)

// memo is what a scene keeps between snapshots so a snapshot lays out
// only what changed in it (item 197): the plans last made and the
// groups' orders last read. A nil memo builds from nothing, which is
// what Build does and what the scene's city is tested against.
type memo struct {
	plans     []planned
	orders    group.Orders
	plansMade int
	// planID is the id of the plan last handed out.
	planID int
	// walks are the avenues' walks between driveways, kept for the plan
	// the city stands on, and walksRouted how many were walked rather
	// than remembered (item 206).
	walks       map[walkKey][]Cell
	walksRouted int
	// lays are the roads last laid for each heading, and laysDone how
	// many times road was laid rather than remembered.
	lays     map[int]laid
	laysDone int
}

// laid is what laying road on the corridors made of a plan's walks.
type laid struct {
	plan   int
	bridge Cell
	paths  []Point
	ends   []int
	cells  []StreetCell
	lamps  []Cell
	trees  []Tree
}

// walkKey is what a walk along the avenues depends on: the plan, the
// heading its ties break toward, and its two ends.
type walkKey struct {
	plan, heading int
	from, to      Cell
}

// planned is a plan and what it was made from: its input, and the
// memory's slots as making it left them.
type planned struct {
	in    plan.Input
	slots map[string]plan.Slot
	p     plan.Plan
	id    int
}

// keptPlans is how many plans are remembered: a city with a depot makes
// two each snapshot.
const keptPlans = 2

// plan is plan.Make, unless the input and the memory are what a plan
// was last made from and left behind. Making a plan writes each
// district's slot into the memory, so memory as it was left is what a
// plan made again from it would read, and would leave unchanged.
func (m *memo) plan(in plan.Input, mem *plan.Memory) plan.Plan {
	if m == nil {
		return plan.Make(in, mem)
	}
	for _, e := range m.plans {
		if sameInput(e.in, in) && maps.Equal(e.slots, mem.Slots) {
			m.planID = e.id
			return e.p
		}
	}
	p := plan.Make(in, mem)
	m.plansMade++
	m.planID = m.plansMade
	in.Districts = slices.Clone(in.Districts)
	m.plans = append(m.plans, planned{in: in, slots: maps.Clone(mem.Slots), p: p, id: m.planID})
	if len(m.plans) > keptPlans {
		m.plans = m.plans[len(m.plans)-keptPlans:]
	}
	return p
}

func sameInput(a, b plan.Input) bool {
	return a.Towers == b.Towers && a.DepotRows == b.DepotRows && a.DepotCols == b.DepotCols && slices.Equal(a.Districts, b.Districts)
}

// current is the id of the plan last handed out, or none without a memo.
func (m *memo) current() int {
	if m == nil {
		return 0
	}
	return m.planID
}

// walk is a walk along the avenues from one cell to another.
// A walk depends only on the plan's avenues and the heading, so one is
// remembered for as long as the city stands on the plan it was walked on.
func (m *memo) walk(k walkKey, walk func(from, to map[Cell]bool) []Cell) []Cell {
	if m == nil {
		return walk(map[Cell]bool{k.from: true}, map[Cell]bool{k.to: true})
	}
	if cells, ok := m.walks[k]; ok {
		return cells
	}
	if m.walks == nil {
		m.walks = map[walkKey][]Cell{}
	}
	cells := walk(map[Cell]bool{k.from: true}, map[Cell]bool{k.to: true})
	m.walksRouted++
	m.walks[k] = cells
	return cells
}

// sweepWalks lets go of every walk on a plan other than the city's.
func (m *memo) sweepWalks(planID int) {
	if m == nil {
		return
	}
	maps.DeleteFunc(m.walks, func(k walkKey, _ []Cell) bool { return k.plan != planID })
}

// lay lays road on the city's corridors for its walks. Everything else
// laying reads (the corridors, the island, the plan's lamps and trees)
// comes from the plan, and the bridge's landing from the plan and the
// row the layout keeps, so what was laid last for a heading stands while
// the plan, the landing and every street's path are what they were.
func (m *memo) lay(c *City) {
	if m == nil {
		c.layRoads()
		return
	}
	var paths []Point
	var ends []int
	for _, st := range c.Streets {
		paths = append(paths, st.Path...)
		ends = append(ends, len(paths))
	}
	if l, ok := m.lays[c.heading]; ok && l.plan == c.planID && l.bridge == c.bridgeAt && slices.Equal(l.paths, paths) && slices.Equal(l.ends, ends) {
		c.StreetCells, c.Lamps, c.Trees = slices.Clone(l.cells), slices.Clone(l.lamps), slices.Clone(l.trees)
		return
	}
	c.layRoads()
	m.laysDone++
	if m.lays == nil {
		m.lays = map[int]laid{}
	}
	m.lays[c.heading] = laid{plan: c.planID, bridge: c.bridgeAt, paths: paths, ends: ends,
		cells: slices.Clone(c.StreetCells), lamps: slices.Clone(c.Lamps), trees: slices.Clone(c.Trees)}
}

// order is group.Order, remembered by group when there is a memo.
func (m *memo) order(key string, sites []group.OrderSite) ([]int, []group.Wait) {
	if m == nil {
		return group.Order(sites)
	}
	return m.orders.Order(key, sites)
}
