package city

import (
	"math"
	"sort"

	"botropolis.city/botropolis/pkg/plan"
)

// Streets are the roads between districts laid on the building-cell grid
// and autotiled: each cell knows which of its four neighbours it joins,
// so the renderer can pick a straight, a corner, a junction or an end.
//
// Directions use the world axes: E is +x, W is -x, S is +y, N is -y.
const (
	DirN = 1 << iota
	DirE
	DirS
	DirW
)

type Cell struct{ Col, Row int }

type StreetCell struct {
	Cell Cell
	Mask int
}

// Street is one road's path as cell centres, kerb to kerb, for traffic.
type Street struct {
	Road *RoadLine
	Path []Point
}

// CellOf is the cell a world point falls in.
func CellOf(p Point) Cell {
	return Cell{Col: int(math.Floor(p.X / BuildingSize)), Row: int(math.Floor(p.Y / BuildingSize))}
}

func (c Cell) Rect() Rect {
	return RectAt(float64(c.Col)*BuildingSize, float64(c.Row)*BuildingSize, BuildingSize, BuildingSize)
}

func (c Cell) Center() Point {
	return c.Rect().Center()
}

// placeStreets lays the plan's avenues as street cells and routes each
// road's traffic along them from one district's kerb to the other's.
func (c *City) placeStreets() {
	c.Streets = nil
	c.StreetCells = nil
	for _, st := range c.plan.Streets {
		c.StreetCells = append(c.StreetCells, StreetCell{Cell: toCell(st.Cell), Mask: st.Mask})
	}
	for i := range c.Roads {
		road := &c.Roads[i]
		cells := c.routeStreets(road)
		var path []Point
		for _, cell := range cells {
			path = append(path, cell.Center())
		}
		if len(path) >= 2 {
			c.Streets = append(c.Streets, Street{Road: road, Path: path})
		}
	}
}

// kerbCells is every street cell touching a district's block.
func (c *City) kerbCells(d *District) map[Cell]bool {
	kerb := map[Cell]bool{}
	min, max := CellOf(d.Rect.Min), CellOf(d.Rect.Max.Sub(Point{X: 1, Y: 1}))
	for col := min.Col - 1; col <= max.Col+1; col++ {
		for row := min.Row - 1; row <= max.Row+1; row++ {
			edge := col < min.Col || col > max.Col || row < min.Row || row > max.Row
			corner := (col < min.Col || col > max.Col) && (row < min.Row || row > max.Row)
			if edge && !corner && c.plan.IsStreet(fromCell(Cell{col, row})) {
				kerb[Cell{col, row}] = true
			}
		}
	}
	return kerb
}

// routeStreets is the shortest walk along the avenues from one
// district's kerb to the other's; a turn costs a little so traffic keeps
// to one avenue where it can.
func (c *City) routeStreets(road *RoadLine) []Cell {
	if road.From == nil || road.To == nil {
		return nil
	}
	from, to := c.kerbCells(road.From), c.kerbCells(road.To)
	if len(from) == 0 || len(to) == 0 {
		return nil
	}
	// Blocks across one avenue share its cells as kerb: the traffic runs
	// along that stretch of avenue rather than nowhere at all.
	var shared []Cell
	for cell := range from {
		if to[cell] {
			shared = append(shared, cell)
		}
	}
	if len(shared) > 0 {
		sort.Slice(shared, func(i, j int) bool { return lessCell(shared[i], shared[j]) })
		return shared
	}
	type entry struct {
		node routeNode
		cost float64
	}
	best := map[routeNode]float64{}
	prev := map[routeNode]routeNode{}
	var frontier []entry
	for cell := range from {
		frontier = append(frontier, entry{routeNode{cell, 0}, 0})
		best[routeNode{cell, 0}] = 0
	}
	sort.Slice(frontier, func(i, j int) bool { return lessCell(frontier[i].node.cell, frontier[j].node.cell) })
	steps := []struct {
		dc, dr, dir int
	}{{1, 0, DirE}, {-1, 0, DirW}, {0, 1, DirS}, {0, -1, DirN}}
	for len(frontier) > 0 {
		k := 0
		for i := range frontier {
			if frontier[i].cost < frontier[k].cost {
				k = i
			}
		}
		cur := frontier[k]
		frontier = append(frontier[:k], frontier[k+1:]...)
		if cur.cost > best[cur.node] {
			continue
		}
		if to[cur.node.cell] {
			return unwind(prev, cur.node)
		}
		for _, st := range steps {
			next := routeNode{Cell{cur.node.cell.Col + st.dc, cur.node.cell.Row + st.dr}, st.dir}
			if !c.plan.IsStreet(fromCell(next.cell)) {
				continue
			}
			cost := cur.cost + 1
			if cur.node.dir != 0 && cur.node.dir != st.dir {
				cost += turnCost
			}
			if old, seen := best[next]; seen && old <= cost {
				continue
			}
			best[next] = cost
			prev[next] = cur.node
			frontier = append(frontier, entry{next, cost})
		}
	}
	return nil
}

func lessCell(a, b Cell) bool {
	if a.Row != b.Row {
		return a.Row < b.Row
	}
	return a.Col < b.Col
}

const turnCost = 0.6

type routeNode struct {
	cell Cell
	dir  int
}

func unwind(prev map[routeNode]routeNode, end routeNode) []Cell {
	var cells []Cell
	for cur := end; ; {
		cells = append(cells, cur.cell)
		p, ok := prev[cur]
		if !ok {
			break
		}
		cur = p
	}
	for i, j := 0, len(cells)-1; i < j; i, j = i+1, j-1 {
		cells[i], cells[j] = cells[j], cells[i]
	}
	return cells
}

func fromCell(c Cell) plan.Cell {
	return plan.Cell{Col: c.Col, Row: c.Row}
}
