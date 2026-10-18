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

// placeStreets keeps the plan's avenues as corridors, routes each
// connection along them from one lot's driveway to the other's, and
// lays road only on the corridor cells a route crosses. Items 85, 108.
func (c *City) placeStreets() {
	c.Streets = nil
	c.StreetCells = nil
	c.Corridors = nil
	c.Lamps = append([]Cell(nil), c.planLamps...)
	c.Trees = append([]Tree(nil), c.planTrees...)
	for _, st := range c.plan.Streets {
		c.Corridors = append(c.Corridors, StreetCell{Cell: toCell(st.Cell), Mask: st.Mask})
	}
	defer c.layRoads()
	c.chooseDriveways()
	for i := range c.Roads {
		road := &c.Roads[i]
		if path := c.routeStreets(road); len(path) >= 2 {
			c.Streets = append(c.Streets, Street{Road: road, Path: path})
		}
	}
}

// Face places the streets again for the camera at a heading, so every
// driveway is on the side of its block the camera now faces.
func (c *City) Face(heading int) {
	heading = (heading%360 + 360) % 360
	if heading == c.heading {
		return
	}
	c.heading = heading
	if c.plan.Streets != nil {
		c.placeStreets()
	}
}

// Heading is the camera heading the streets are placed for.
func (c *City) Heading() int { return c.heading }

// layRoads lays a road on every corridor cell a route crosses, each cell
// joined only to the road cells beside it and a driveway to its lot, and
// keeps the lamps and the street trees to the roads: a corridor nothing
// travels is belt.
func (c *City) layRoads() {
	road := map[Cell]bool{}
	stub := map[Cell]int{}
	for _, street := range c.Streets {
		path := street.Path
		for _, p := range path[1 : len(path)-1] {
			road[CellOf(p)] = true
		}
		for _, end := range [][2]Point{{path[1], path[0]}, {path[len(path)-2], path[len(path)-1]}} {
			cell := CellOf(end[0])
			stub[cell] |= sideOf(cell, end[1])
		}
	}
	// A road that reaches the bridge's landing joins it: a T, not a
	// road running past the bridge's end (Aria, 2026-10-18).
	if land, ok := c.bridgeCell(); ok && road[land] {
		toward := DirE
		if c.Bridge().Far.X < c.Bridge().Land.X {
			toward = DirW
		}
		stub[land] |= toward
	}
	for _, corridor := range c.Corridors {
		if road[corridor.Cell] {
			c.StreetCells = append(c.StreetCells, StreetCell{Cell: corridor.Cell, Mask: roadMask(corridor.Cell, road) | stub[corridor.Cell]})
		}
	}
	lamps := c.Lamps[:0]
	for _, l := range c.Lamps {
		if road[l] {
			lamps = append(lamps, l)
		}
	}
	c.Lamps = lamps
	island := c.Island()
	trees := c.Trees[:0]
	for _, t := range c.Trees {
		g, _ := island.At(t.At)
		switch {
		case t.Kind == plan.ParkTree && g == GroundRiver:
			// The river's banks wander past its band; no tree stands in it.
		case t.Kind == plan.StreetTree && !besideRoad(t.Cell, road):
		case t.Kind == plan.ParkTree && road[CellOf(t.At)]:
			// The belt's wood on a corridor gives way to the road.
		default:
			trees = append(trees, t)
		}
	}
	c.Trees = trees
}

var sides = []struct{ dc, dr, dir int }{{0, -1, DirN}, {1, 0, DirE}, {0, 1, DirS}, {-1, 0, DirW}}

func roadMask(cell Cell, road map[Cell]bool) int {
	mask := 0
	for _, s := range sides {
		if road[Cell{cell.Col + s.dc, cell.Row + s.dr}] {
			mask |= s.dir
		}
	}
	return mask
}

func besideRoad(cell Cell, road map[Cell]bool) bool {
	return roadMask(cell, road) != 0
}

// sideOf is the side of a cell a point lies off, as a direction.
func sideOf(cell Cell, p Point) int {
	d := p.Sub(cell.Center())
	switch {
	case math.Abs(d.X) >= math.Abs(d.Y) && d.X > 0:
		return DirE
	case math.Abs(d.X) >= math.Abs(d.Y):
		return DirW
	case d.Y > 0:
		return DirS
	}
	return DirN
}

// front is the way out of a block towards the camera, in cells: south
// before the camera is turned, and a quarter round for each turn.
func (c *City) front() (int, int) {
	switch c.heading / 90 {
	case 1:
		return 1, 0
	case 2:
		return 0, -1
	case 3:
		return -1, 0
	}
	return 0, 1
}

// depth is how near the camera a cell is, in cells: larger is nearer.
func (c *City) depth(cell Cell) int {
	dc, dr := c.front()
	// The camera's own frame puts depth along x + y: Camera.turn, read
	// off for each heading.
	return cell.Col*(dc+dr) + cell.Row*(dr-dc)
}

// Driveway is where a lot meets the street: the avenue cell under the
// middle of its block's camera-facing side, and the lot end just inside
// the block that a road starts from. Item 108.
type Driveway struct {
	Cell Cell
	Lot  Point
	// side and at are where it is on its block, for the layout to keep.
	side [2]int
	at   int
}

// Driveway is a district's driveway, if its block has one: the side
// chosen for its roads when the streets were placed, and otherwise the
// middle of the side facing the camera.
func (c *City) Driveway(d *District) (Driveway, bool) {
	if dw, ok := c.driveways[d]; ok {
		return dw, true
	}
	dc, dr := c.front()
	return c.drivewayOn(d, dc, dr)
}

// drivewayOn is a district's driveway on one side of its block: the
// avenue cell nearest the middle of that side, an even side's middle
// rounding toward the camera.
func (c *City) drivewayOn(d *District, dc, dr int) (Driveway, bool) {
	return c.drivewayNear(d, dc, dr, -1)
}

// drivewayNear is a district's driveway on one side of its block, at
// the avenue cell nearest a place along that side, counted in cells
// from its west or north end; a negative place is the middle.
func (c *City) drivewayNear(d *District, dc, dr, at int) (Driveway, bool) {
	if d == nil {
		return Driveway{}, false
	}
	min, max := CellOf(d.Rect.Min), CellOf(d.Rect.Max.Sub(Point{X: 1, Y: 1}))
	var along []Cell
	if dr != 0 {
		row := max.Row + 1
		if dr < 0 {
			row = min.Row - 1
		}
		for col := min.Col; col <= max.Col; col++ {
			along = append(along, Cell{col, row})
		}
	} else {
		col := max.Col + 1
		if dc < 0 {
			col = min.Col - 1
		}
		for row := min.Row; row <= max.Row; row++ {
			along = append(along, Cell{col, row})
		}
	}
	target := float64(len(along)-1) / 2
	if at >= 0 {
		target = float64(at)
	}
	index := map[Cell]int{}
	for i, cell := range along {
		index[cell] = i
	}
	sort.SliceStable(along, func(i, j int) bool {
		di, dj := math.Abs(float64(index[along[i]])-target), math.Abs(float64(index[along[j]])-target)
		if di != dj {
			return di < dj
		}
		return c.depth(along[i]) > c.depth(along[j])
	})
	for _, cell := range along {
		if !c.plan.IsStreet(fromCell(cell)) {
			continue
		}
		lot := cell.Center()
		switch {
		case dr > 0:
			lot.Y = d.Rect.Max.Y - DoorInset
		case dr < 0:
			lot.Y = d.Rect.Min.Y + DoorInset
		case dc > 0:
			lot.X = d.Rect.Max.X - DoorInset
		default:
			lot.X = d.Rect.Min.X + DoorInset
		}
		return Driveway{Cell: cell, Lot: lot, side: [2]int{dc, dr}, at: index[cell]}, true
	}
	return Driveway{}, false
}

// drivewaySlack is how many cells shorter a lot's roads must be by
// another side before its driveway leaves the side facing the camera
// (Aria, 2026-10-18: "within a cell or two").
const drivewaySlack = 2

// chooseDriveways gives each lot with roads the side that keeps the
// total length of its roads shortest, the camera-facing side unless
// another is more than drivewaySlack cells shorter, settling each lot
// against the others' choices until none moves. Item 108.
//
// A lot keeps the driveway it was first given, through new roads, turns
// of the camera and restarts (Aria, 2026-10-18: "We don't want driveways
// moving"), so only a lot with roads and no driveway yet is chosen for.
func (c *City) chooseDriveways() {
	c.driveways = map[*District]Driveway{}
	partners := map[*District][]*District{}
	for i := range c.Roads {
		r := &c.Roads[i]
		if r.From == nil || r.To == nil || r.From == r.To {
			continue
		}
		partners[r.From] = append(partners[r.From], r.To)
		partners[r.To] = append(partners[r.To], r.From)
	}
	fc, fr := c.front()
	type side struct {
		dw       Driveway
		facing   bool
		distance map[Cell]int
	}
	sides := map[*District][]side{}
	var order []*District
	for _, d := range c.Districts {
		if dc, dr, at, kept := c.keptDriveway(d); kept {
			if dw, ok := c.drivewayNear(d, dc, dr, at); ok {
				c.driveways[d] = dw
				continue
			}
		}
		if front, ok := c.drivewayOn(d, fc, fr); ok {
			c.driveways[d] = front
		}
		if len(partners[d]) == 0 {
			continue
		}
		order = append(order, d)
		for _, s := range []struct{ dc, dr int }{{fc, fr}, {-fr, fc}, {fr, -fc}, {-fc, -fr}} {
			if dw, ok := c.drivewayOn(d, s.dc, s.dr); ok {
				sides[d] = append(sides[d], side{dw: dw, facing: s.dc == fc && s.dr == fr, distance: c.streetDistances(dw.Cell)})
			}
		}
	}
	const far = 1 << 20
	for round := 0; round < 8; round++ {
		moved := false
		for _, d := range order {
			total := func(s side) int {
				sum := 0
				for _, p := range partners[d] {
					n, ok := s.distance[c.driveways[p].Cell]
					if !ok {
						n = far
					}
					sum += n
				}
				return sum
			}
			best, bestTotal, facing, facingTotal := -1, far*len(partners[d]), -1, 0
			for i, s := range sides[d] {
				n := total(s)
				if s.facing {
					facing, facingTotal = i, n
				}
				if n < bestTotal || (n == bestTotal && best >= 0 && c.depth(s.dw.Cell) > c.depth(sides[d][best].dw.Cell)) {
					best, bestTotal = i, n
				}
			}
			if best < 0 {
				continue
			}
			pick := sides[d][best].dw
			if facing >= 0 && facingTotal-bestTotal <= drivewaySlack {
				pick = sides[d][facing].dw
			}
			if pick != c.driveways[d] {
				c.driveways[d], moved = pick, true
			}
		}
		if !moved {
			break
		}
	}
	if c.layout != nil {
		for _, d := range order {
			if dw, ok := c.driveways[d]; ok {
				c.layout.keepDriveway(d.Root, dw.side[0], dw.side[1], dw.at)
			}
		}
	}
}

// keptDriveway is the driveway the layout keeps for a district, if any.
func (c *City) keptDriveway(d *District) (dc, dr, at int, ok bool) {
	if c.layout == nil {
		return 0, 0, 0, false
	}
	return c.layout.keptDriveway(d.Root)
}

// streetDistances is how many steps along the avenues every street cell
// is from one.
func (c *City) streetDistances(from Cell) map[Cell]int {
	dist := map[Cell]int{from: 0}
	queue := []Cell{from}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, s := range sides {
			next := Cell{cur.Col + s.dc, cur.Row + s.dr}
			if _, seen := dist[next]; seen || !c.plan.IsStreet(fromCell(next)) {
				continue
			}
			dist[next] = dist[cur] + 1
			queue = append(queue, next)
		}
	}
	return dist
}

// routeStreets is a road's way from one lot to the other: out of the
// first lot by its driveway, the cheapest walk along the avenues to the
// other's driveway, and into that lot. Item 108.
func (c *City) routeStreets(road *RoadLine) []Point {
	if road.From == nil || road.To == nil {
		return nil
	}
	from, ok := c.Driveway(road.From)
	to, ok2 := c.Driveway(road.To)
	if !ok || !ok2 {
		return nil
	}
	cells := c.walkStreets(map[Cell]bool{from.Cell: true}, map[Cell]bool{to.Cell: true})
	if len(cells) == 0 {
		return nil
	}
	path := []Point{from.Lot}
	for _, cell := range cells {
		path = append(path, cell.Center())
	}
	return append(path, to.Lot)
}

// walkStreets is the cheapest walk along the avenues from any of one set
// of cells to any of another, or nil when none joins them.
func (c *City) walkStreets(from, to map[Cell]bool) []Cell {
	type entry struct {
		node routeNode
		cost float64
	}
	best := map[routeNode]float64{}
	prev := map[routeNode]routeNode{}
	nearest := math.MinInt
	for _, st := range c.plan.Streets {
		nearest = max(nearest, c.depth(toCell(st.Cell)))
	}
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
			cost := cur.cost + 1 + tieCost*float64(nearest-c.depth(next.cell))
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

// tieCost settles two walks of the same length and turns toward the
// camera, a cell nearer it costing less. It is far below any difference
// steps and turns can make: a step and a turn differ by 0.2 at the
// least, and a walk across the widest plan adds well under that.
const tieCost = 1e-6

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

// Pads is the paved strip from each driveway among the cells into its
// lot: every side a cell is joined on that is not another street.
func (c *City) Pads(cells []StreetCell) []Rect {
	var out []Rect
	for _, sc := range cells {
		for _, s := range sides {
			lot := Cell{sc.Cell.Col + s.dc, sc.Cell.Row + s.dr}
			if sc.Mask&s.dir == 0 || c.plan.IsStreet(fromCell(lot)) || !c.inLot(lot) {
				continue
			}
			out = append(out, padInto(sc.Cell, s.dc, s.dr))
		}
	}
	return out
}

// PadWidth and PadDepth are a driveway pad's size: about the road's own
// width, and most of the lot's padding deep, short of the buildings.
const (
	PadWidth = 1.75 * Tile
	PadDepth = 0.75 * DistrictPadding
)

// padInto is the pad off one side of a street cell, into the lot there.
func padInto(cell Cell, dc, dr int) Rect {
	r := cell.Rect()
	mid := r.Center()
	half := PadWidth / 2
	switch {
	case dr > 0:
		return Rect{Min: Point{X: mid.X - half, Y: r.Max.Y}, Max: Point{X: mid.X + half, Y: r.Max.Y + PadDepth}}
	case dr < 0:
		return Rect{Min: Point{X: mid.X - half, Y: r.Min.Y - PadDepth}, Max: Point{X: mid.X + half, Y: r.Min.Y}}
	case dc > 0:
		return Rect{Min: Point{X: r.Max.X, Y: mid.Y - half}, Max: Point{X: r.Max.X + PadDepth, Y: mid.Y + half}}
	}
	return Rect{Min: Point{X: r.Min.X - PadDepth, Y: mid.Y - half}, Max: Point{X: r.Min.X, Y: mid.Y + half}}
}

// inLot is whether a cell is inside a district's block.
func (c *City) inLot(cell Cell) bool {
	for _, d := range c.Districts {
		if d.Rect.Contains(cell.Center()) {
			return true
		}
	}
	return false
}
