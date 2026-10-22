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
}

// planned is a plan and what it was made from: its input, and the
// memory's slots as making it left them.
type planned struct {
	in    plan.Input
	slots map[string]plan.Slot
	p     plan.Plan
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
			return e.p
		}
	}
	p := plan.Make(in, mem)
	m.plansMade++
	in.Districts = slices.Clone(in.Districts)
	m.plans = append(m.plans, planned{in: in, slots: maps.Clone(mem.Slots), p: p})
	if len(m.plans) > keptPlans {
		m.plans = m.plans[len(m.plans)-keptPlans:]
	}
	return p
}

func sameInput(a, b plan.Input) bool {
	return a.Towers == b.Towers && a.DepotRows == b.DepotRows && a.DepotCols == b.DepotCols && slices.Equal(a.Districts, b.Districts)
}

// order is group.Order, remembered by group when there is a memo.
func (m *memo) order(key string, sites []group.OrderSite) ([]int, []group.Wait) {
	if m == nil {
		return group.Order(sites)
	}
	return m.orders.Order(key, sites)
}
