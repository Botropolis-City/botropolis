// Package plan is the city plan: a pure function from what the city holds
// to where everything goes, in cells. A central plaza, rings of blocks
// around it on an avenue grid, parks in the blocks nobody uses, a belt of
// park around the lot, a telecom ridge along the north, storage along the
// south and the river down the east edge. Every decoration comes from
// here, so every tree can be pointed at.
package plan

import (
	"fmt"
	"hash/fnv"
	"sort"
)

// Cell is one grid cell; the city decides how many pixels that is.
type Cell struct {
	Col, Row int
}

// Block is a rectangle of cells.
type Block struct {
	Min        Cell
	Cols, Rows int
}

func (b Block) Contains(c Cell) bool {
	return c.Col >= b.Min.Col && c.Col < b.Min.Col+b.Cols && c.Row >= b.Min.Row && c.Row < b.Min.Row+b.Rows
}

func (b Block) Center() Cell {
	return Cell{Col: b.Min.Col + b.Cols/2, Row: b.Min.Row + b.Rows/2}
}

// Slot is a block's place on the rings: (0,0) is the plaza and the ring
// is the Chebyshev distance from it.
type Slot struct {
	Col int `json:"col"`
	Row int `json:"row"`
}

func (s Slot) Ring() int {
	return max(abs(s.Col), abs(s.Row))
}

// District is a live project and the cells its buildings need.
type District struct {
	Root       string
	Cols, Rows int
}

// Input is everything the plan is made from.
type Input struct {
	Districts []District
	Towers    int
	// DepotRows is how many rows the depot of packed-up sessions needs
	// along the south, or none, and DepotCols how many columns, from the
	// plan's west edge; none is the plan's width (item 86).
	DepotRows int
	DepotCols int
}

// Memory is what the plan remembers between snapshots: each district's
// slot, so a block stays put until its status changes.
type Memory struct {
	Slots map[string]Slot `json:"slots"`
}

func NewMemory() *Memory {
	return &Memory{Slots: map[string]Slot{}}
}

// Directions on the world axes: E is +col, S is +row.
const (
	DirN = 1 << iota
	DirE
	DirS
	DirW
)

// TreeKind is what a planting is, and how the city draws it: a tree in
// a park or the belt wood, one of a line along an avenue, or a bush or
// planter on the plaza's edge.
type TreeKind int

const (
	ParkTree TreeKind = iota
	StreetTree
	Bush
	Planter
)

// How many species the renderer keeps for each kind; the plan picks one
// by index and pkg/render maps it to a piece. A park mixes two to four
// of the park species, a street is planted in one all the way down.
const (
	ParkSpecies   = 8
	StreetSpecies = 3
	BushSpecies   = 2
)

// Tree is one planting: its cell, what kind it is, which species, and
// how far off the cell's centre it stands, in cells, so a block of park
// reads as a wood rather than a hedge. Everything is seeded from the
// cell or its block, so the same plan grows the same garden every time
// and nothing moves between frames.
type Tree struct {
	Cell    Cell
	Kind    TreeKind
	Variant int
	DX, DY  float64
}

// TreeJitter is how far a tree may stand from its cell's centre, in cells.
const TreeJitter = 0.3

// KerbOffset is how far a street tree stands from the centre of its
// verge cell, back towards the kerb it lines.
//
// Bug 41. This used to be an offset within the *street* cell, and its
// comment said "out on the verge, clear of the carriageway" — but a
// street cell is carriageway edge to edge and has no verge, and 0.42 is
// less than the half-cell of 0.5, so it never left the road. Every
// street tree in the city stood on tarmac against the property line,
// which is what "trees land in concrete at the edge of properties"
// was. The comment described a verge that was never built. The tree now
// stands on the cell across the kerb, and this offset leans it back
// towards the street from there.
const KerbOffset = 0.42

// RiverCols is how many cells wide the river runs.
//
// Derived rather than chosen, but from a different requirement than it
// used to be. It was two cells because the usage boats read their
// percentage ACROSS the river and the spread had to stay legible at
// MinZoom. Bug 52 put the reading along the river instead, which is
// what Aria asked for, so that derivation no longer holds up anything
// — leaving it here would be a number that is right and means nothing.
//
// What sets the width now is three hulls abreast. The boats are held
// apart in a lane each, and keeping the outer two off the banks leaves
// a lane spacing of (width - widest hull) / 2. Measured off the cut
// sprites at z1, the beams are 47.7, 19.8 and 17.2 world units, and the
// binding pair is the big boat beside the medium one, which needs 33.8
// between lane centres. Two cells give 24.1 — the liner overlaps the
// cargo ship by nearly ten units at every zoom and every heading, not
// just when the readings converge. Three cells give 48.1, clearing it
// by 14.4 units, which is 22 px at zoom 1 and 5.5 px at MinZoom.
//
// Aria pre-authorised exactly this: "We can make the river wider if
// needed to fit 3 boats."
//
// pkg/render's TestRiverFitsThreeHullsAbreast re-derives the beams from
// the atlas and fails if this width stops clearing them, so the river
// and the hulls cannot drift apart.
const RiverCols = 3

// StreetTreeSpacing is how many cells apart street trees stand. A city
// plants a street at a fixed pitch, not wherever there is room.
const StreetTreeSpacing = 3

func seedOf(parts ...int) uint32 {
	h := fnv.New32a()
	for _, p := range parts {
		_, _ = fmt.Fprintf(h, "%d,", p)
	}
	return h.Sum32()
}

func unitOf(bits uint32) float64 { return float64(bits&0xffff)/0xffff*2 - 1 }

// grove is the two to four species a park block is planted with, drawn
// from the park palette by the block's own corner.
func grove(b Block) []int {
	seed := seedOf(b.Min.Col, b.Min.Row, b.Cols, b.Rows)
	n := 2 + int(seed%3)
	var species []int
	for i := 0; len(species) < n; i++ {
		pick := int((seed>>(2*uint(i))+uint32(i)*7)%ParkSpecies) % ParkSpecies
		if !contains(species, pick) {
			species = append(species, pick)
		}
		if i > 3*ParkSpecies {
			break
		}
	}
	return species
}

func contains(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// everySpecies is the whole park palette, for the belt: the wood is
// mixed rather than planted in groves.
func everySpecies() []int {
	all := make([]int, ParkSpecies)
	for i := range all {
		all[i] = i
	}
	return all
}

// Street is one avenue cell and which neighbours it joins.
type Street struct {
	Cell Cell
	Mask int
}

// RiverCell is one cell of river and which neighbours it flows to.
type RiverCell struct {
	Cell Cell
	Mask int
}

// Plan is the city laid out.
// Plots are the plaza's reserved corners: the ground the civic
// buildings stand on.
//
// They belong to the plan because the plan is what decides where
// anything goes, and because decoration cannot keep off what it does not
// know about. The plaza used to have two deciders and no referee — the
// plan planted a bush or a planter on every cell of the rim, and the
// city dropped the plant, the hall and the library onto the same plaza
// afterwards, computed from the plaza's own rectangle. Nothing made the
// two agree, so planters grew out of the concrete and a bush floated
// beside the cooling tower.
type Plots struct {
	Plant   Block
	Hall    Block
	Library Block
}

// All is the three plots, for asking whether a cell is spoken for.
func (p Plots) All() []Block { return []Block{p.Plant, p.Hall, p.Library} }

// Taken reports whether a cell falls on any reserved plot.
func (p Plots) Taken(c Cell) bool {
	for _, b := range p.All() {
		if b.Contains(c) {
			return true
		}
	}
	return false
}

type Plan struct {
	Bounds Block
	Plaza  Block
	// Plots is the plaza's reserved ground; see Plots.
	Plots    Plots
	Blocks   map[string]Block
	Slots    map[string]Slot
	Parks    []Block
	Belt     []Block
	Depot    Block
	Towers   []Cell
	Streets  []Street
	Lamps    []Cell
	Trees    []Tree
	Fountain Cell
	River    []RiverCell
	// Rails is the freight loop, cell by cell clockwise from the
	// north-west corner of the inner belt: the ledger's line, which every
	// district lies inside.
	Rails []Cell

	rings  int
	span   int
	colX   map[int]int
	rowY   map[int]int
	width  map[int]int
	height map[int]int
	street map[Cell]int
	park   map[Cell]bool
}

const (
	minSlot   = 3
	plazaCols = 6
	plazaRows = 5
	belt      = 2
)

// Make lays the plan out, keeping every remembered slot it can.
func Make(in Input, mem *Memory) Plan {
	if mem == nil {
		mem = NewMemory()
	}
	p := Plan{Blocks: map[string]Block{}, Slots: map[string]Slot{}, street: map[Cell]int{}, park: map[Cell]bool{}}
	p.assign(in.Districts, mem)
	p.measure(in.Districts)
	p.place(in)
	return p
}

// assign gives every district a slot: the remembered one when it is free,
// else the first free slot on the innermost ring.
func (p *Plan) assign(districts []District, mem *Memory) {
	sorted := append([]District(nil), districts...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Root < sorted[j].Root })
	taken := map[Slot]bool{}
	var pending []District
	p.rings = 1
	for _, d := range sorted {
		slot, ok := mem.Slots[d.Root]
		if !ok || slot.Ring() < 1 || taken[slot] {
			pending = append(pending, d)
			continue
		}
		taken[slot] = true
		p.Slots[d.Root] = slot
		p.rings = max(p.rings, slot.Ring())
	}
	for _, d := range pending {
		for ring := 1; ; ring++ {
			slot, ok := freeSlot(ring, taken)
			if ok {
				taken[slot] = true
				p.Slots[d.Root] = slot
				p.rings = max(p.rings, ring)
				break
			}
		}
	}
	for root, slot := range p.Slots {
		mem.Slots[root] = slot
	}
}

// ringSlots is every slot on a ring, north-west corner first, row by row.
func ringSlots(ring int) []Slot {
	var slots []Slot
	for row := -ring; row <= ring; row++ {
		for col := -ring; col <= ring; col++ {
			if (Slot{col, row}).Ring() == ring {
				slots = append(slots, Slot{col, row})
			}
		}
	}
	return slots
}

func freeSlot(ring int, taken map[Slot]bool) (Slot, bool) {
	for _, s := range ringSlots(ring) {
		if !taken[s] {
			return s, true
		}
	}
	return Slot{}, false
}

// measure sizes every column and row of slots to the largest block in it,
// so avenues run straight.
func (p *Plan) measure(districts []District) {
	p.width, p.height = map[int]int{}, map[int]int{}
	for c := -p.rings; c <= p.rings; c++ {
		p.width[c] = minSlot
		p.height[c] = minSlot
	}
	p.width[0], p.height[0] = max(minSlot, plazaCols), max(minSlot, plazaRows)
	for _, d := range districts {
		slot := p.Slots[d.Root]
		p.width[slot.Col] = max(p.width[slot.Col], d.Cols)
		p.height[slot.Row] = max(p.height[slot.Row], d.Rows)
	}
}

// place walks the plan from its north-west corner: belt, ridge, ring
// road, the rings with avenues between, ring road, storage, road, belt,
// and the river beyond the east belt.
func (p *Plan) place(in Input) {
	p.colX, p.rowY = map[int]int{}, map[int]int{}
	x := belt + 1
	westRoad := belt
	for c := -p.rings; c <= p.rings; c++ {
		p.colX[c] = x
		x += p.width[c] + 1
	}
	eastRoad := x - 1
	p.span = eastRoad - p.colX[-p.rings]
	cols := eastRoad + 1 + belt + RiverCols

	ridge := belt
	y := ridge + 1 + 1
	northRoad := ridge + 1
	for r := -p.rings; r <= p.rings; r++ {
		p.rowY[r] = y
		y += p.height[r] + 1
	}
	southRoad := y - 1
	lastRoad := southRoad
	if in.DepotRows > 0 {
		cols := eastRoad - p.colX[-p.rings]
		if in.DepotCols > 0 && in.DepotCols < cols {
			cols = in.DepotCols
		}
		p.Depot = Block{Min: Cell{Col: p.colX[-p.rings], Row: southRoad + 1}, Cols: cols, Rows: in.DepotRows}
		lastRoad = southRoad + 1 + in.DepotRows
	}
	rows := lastRoad + 1 + belt
	p.Bounds = Block{Cols: cols, Rows: rows}

	p.Plaza = p.BlockAt(Slot{})
	p.Fountain = p.Plaza.Center()
	p.Plots = plazaPlots(p.Plaza)
	used := map[Slot]bool{}
	for root, slot := range p.Slots {
		p.Blocks[root] = p.BlockAt(slot)
		used[slot] = true
	}
	for ring := 1; ring <= p.rings; ring++ {
		for _, slot := range ringSlots(ring) {
			if !used[slot] {
				p.Parks = append(p.Parks, p.BlockAt(slot))
			}
		}
	}
	riverCol := cols - RiverCols
	p.Belt = []Block{
		{Min: Cell{}, Cols: riverCol, Rows: belt},
		{Min: Cell{Row: lastRoad + 1}, Cols: riverCol, Rows: belt},
		{Min: Cell{Row: belt}, Cols: belt, Rows: lastRoad + 1 - belt},
		{Min: Cell{Col: eastRoad + 1, Row: belt}, Cols: belt, Rows: lastRoad + 1 - belt},
	}
	// The west leg stands two cells off its road, as the north leg does
	// beyond the ridge, so the bridge road ramps up before it crosses
	// the rail (item 146).
	p.Rails = railLoop(Cell{Col: westRoad - 2, Row: northRoad - 2}, Cell{Col: eastRoad + 1, Row: lastRoad + 1})
	onRails := map[Cell]bool{}
	for _, r := range p.Rails {
		onRails[r] = true
	}
	for _, b := range append(append([]Block(nil), p.Parks...), p.Belt...) {
		for row := b.Min.Row; row < b.Min.Row+b.Rows; row++ {
			for col := b.Min.Col; col < b.Min.Col+b.Cols; col++ {
				p.park[Cell{col, row}] = true
			}
		}
	}
	for i := 0; i < in.Towers; i++ {
		p.Towers = append(p.Towers, Cell{Col: westRoad + 1 + 2*i, Row: ridge})
	}
	for row := 0; row < rows; row++ {
		for col := riverCol; col < riverCol+RiverCols; col++ {
			mask := DirN | DirS
			if row == 0 {
				mask = DirS
			}
			if row == rows-1 {
				mask = DirN
			}
			if col > riverCol {
				mask |= DirW
			}
			if col < riverCol+RiverCols-1 {
				mask |= DirE
			}
			p.River = append(p.River, RiverCell{Cell: Cell{col, row}, Mask: mask})
		}
	}

	streetCols := []int{westRoad}
	for c := -p.rings; c <= p.rings; c++ {
		streetCols = append(streetCols, p.colX[c]+p.width[c])
	}
	streetRows := []int{northRoad}
	for r := -p.rings; r <= p.rings; r++ {
		streetRows = append(streetRows, p.rowY[r]+p.height[r])
	}
	if in.DepotRows > 0 {
		streetRows = append(streetRows, lastRoad)
	}
	for _, col := range streetCols {
		for row := northRoad; row <= lastRoad; row++ {
			p.street[Cell{col, row}] = 0
		}
	}
	for _, row := range streetRows {
		for col := westRoad; col <= eastRoad; col++ {
			p.street[Cell{col, row}] = 0
		}
	}
	for cell := range p.street {
		mask := 0
		for _, n := range []struct {
			dc, dr, dir int
		}{{0, -1, DirN}, {1, 0, DirE}, {0, 1, DirS}, {-1, 0, DirW}} {
			if _, ok := p.street[Cell{cell.Col + n.dc, cell.Row + n.dr}]; ok {
				mask |= n.dir
			}
		}
		p.street[cell] = mask
	}
	for cell, mask := range p.street {
		p.Streets = append(p.Streets, Street{Cell: cell, Mask: mask})
		if mask == DirN|DirE|DirS|DirW {
			p.Lamps = append(p.Lamps, cell)
		}
	}
	p.plantGround(onRails)
	p.plantStreets()
	p.plantPlazaEdge()
	sort.Slice(p.Streets, func(i, j int) bool { return less(p.Streets[i].Cell, p.Streets[j].Cell) })
	sort.Slice(p.Lamps, func(i, j int) bool { return less(p.Lamps[i], p.Lamps[j]) })
	sort.SliceStable(p.Trees, func(i, j int) bool { return less(p.Trees[i].Cell, p.Trees[j].Cell) })
}

// plantStreets lines every avenue with trees at a fixed pitch, out on
// the verge and only along the straights, so nothing stands in a
// junction. A street is planted in one species the whole way down.
// verge is the cell a street tree stands on: the one across the kerb on
// the given side, or the one opposite if that side is built on. It
// reports false when neither side is ground a tree could stand on.
func (p *Plan) verge(from Cell, dcol, drow int, taken map[Cell]bool) (Cell, bool) {
	for _, try := range [2]Cell{
		{Col: from.Col + dcol, Row: from.Row + drow},
		{Col: from.Col - dcol, Row: from.Row - drow},
	} {
		if !taken[try] && p.plantable(try) {
			return try, true
		}
	}
	return Cell{}, false
}

// plantable reports whether a cell is open ground: not carriageway, not
// inside a block, not the plaza or the storage yard, and not on the
// freight loop.
func (p *Plan) plantable(c Cell) bool {
	if _, road := p.street[c]; road {
		return false
	}
	if p.Plaza.Contains(c) || p.Depot.Contains(c) {
		return false
	}
	for _, b := range p.Blocks {
		if b.Contains(c) {
			return false
		}
	}
	for _, rail := range p.Rails {
		if rail == c {
			return false
		}
	}
	return true
}

func (p *Plan) plantStreets() {
	// In cell order, not map order. Two stretches of street can want
	// the same verge cell, and p.Trees is sorted *stably* by cell, so
	// whoever was visited first won the tie — which a map range makes
	// random. The old code never collided, because every tree stayed on
	// its own street cell, so this only became load-bearing with bug 41.
	cells := make([]Cell, 0, len(p.street))
	for cell := range p.street {
		cells = append(cells, cell)
	}
	sort.Slice(cells, func(i, j int) bool { return less(cells[i], cells[j]) })
	taken := make(map[Cell]bool, len(cells))
	for _, cell := range cells {
		mask := p.street[cell]
		vertical := mask == DirN|DirS
		if !vertical && mask != DirE|DirW {
			continue
		}
		along, line := cell.Row, cell.Col
		if !vertical {
			along, line = cell.Col, cell.Row
		}
		if mod(along, StreetTreeSpacing) != 0 {
			continue
		}
		species := int(seedOf(line, int(b2i(vertical))) % StreetSpecies)
		side := 1.0
		if mod(along, 2*StreetTreeSpacing) != 0 {
			side = -1
		}
		dcol, drow := 0, 0
		if vertical {
			dcol = int(side)
		} else {
			drow = int(side)
		}
		verge, ok := p.verge(cell, dcol, drow, taken)
		if !ok {
			// Nothing to plant on either side: this stretch of street
			// runs between two built cells, and a tree there would be
			// standing in the road. Bug 41.
			continue
		}
		if verge.Col != cell.Col+dcol || verge.Row != cell.Row+drow {
			side = -side
		}
		taken[verge] = true
		tree := Tree{Cell: verge, Kind: StreetTree, Variant: species}
		if vertical {
			tree.DX = -side * KerbOffset
		} else {
			tree.DY = -side * KerbOffset
		}
		p.Trees = append(p.Trees, tree)
	}
}

// plantPlazaEdge sets bushes and planters round the plaza's rim, one
// alternating with the other, so the civic square has a planted edge
// and its middle stays clear for the landmarks.
func (p *Plan) plantPlazaEdge() {
	b := p.Plaza
	if b.Cols < 2 || b.Rows < 2 {
		return
	}
	i := 0
	for row := b.Min.Row; row < b.Min.Row+b.Rows; row++ {
		for col := b.Min.Col; col < b.Min.Col+b.Cols; col++ {
			edge := row == b.Min.Row || row == b.Min.Row+b.Rows-1 ||
				col == b.Min.Col || col == b.Min.Col+b.Cols-1
			if !edge {
				continue
			}
			cell := Cell{col, row}
			// Nothing is planted on ground already spoken for. The
			// plots and the fountain are the plan's own, so this asks
			// the plan rather than guessing from geometry — which is
			// the whole reason the plots exist.
			if p.Plots.Taken(cell) || cell == p.Fountain {
				continue
			}
			seed := seedOf(cell.Col, cell.Row)
			t := Tree{Cell: cell, Kind: Bush, Variant: int(seed>>8) % BushSpecies,
				DX: unitOf(seed>>1) * TreeJitter / 2, DY: unitOf(seed>>17) * TreeJitter / 2}
			if i%2 == 1 {
				t.Kind, t.Variant = Planter, 0
			}
			p.Trees = append(p.Trees, t)
			i++
		}
	}
}

func mod(a, n int) int { return ((a % n) + n) % n }

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// railLoop is every cell of the rectangle from nw to se, clockwise from
// nw, each adjacent to the next and the last to the first.
func railLoop(nw, se Cell) []Cell {
	var loop []Cell
	for col := nw.Col; col < se.Col; col++ {
		loop = append(loop, Cell{Col: col, Row: nw.Row})
	}
	for row := nw.Row; row < se.Row; row++ {
		loop = append(loop, Cell{Col: se.Col, Row: row})
	}
	for col := se.Col; col > nw.Col; col-- {
		loop = append(loop, Cell{Col: col, Row: se.Row})
	}
	for row := se.Row; row > nw.Row; row-- {
		loop = append(loop, Cell{Col: nw.Col, Row: row})
	}
	return loop
}

func less(a, b Cell) bool {
	if a.Row != b.Row {
		return a.Row < b.Row
	}
	return a.Col < b.Col
}

// BlockAt is the block a slot occupies; slots in a column share a width
// and slots in a row share a height.
func (p Plan) BlockAt(s Slot) Block {
	return Block{Min: Cell{Col: p.colX[s.Col], Row: p.rowY[s.Row]}, Cols: p.width[s.Col], Rows: p.height[s.Row]}
}

// RowMark is a row of a plan told by its place among the slot rows: the
// band of slots it falls in, or the road past it, and how many rows
// into that band it is. A band growing or shrinking elsewhere, or a ring
// more, leaves it the same stretch of road beside the same blocks.
type RowMark struct {
	Band   int `json:"band"`
	Offset int `json:"offset"`
}

// MarkRow is row's mark on this plan.
func (p Plan) MarkRow(row int) RowMark {
	band := -p.rings
	for r := -p.rings; r <= p.rings; r++ {
		if p.rowY[r] <= row {
			band = r
		}
	}
	return RowMark{Band: band, Offset: row - p.rowY[band]}
}

// RowOf is the row a mark stands for on this plan. A band the plan no
// longer has stands for the road nearest it: the north road for a band
// north of the rings, the south road for one south of them.
func (p Plan) RowOf(m RowMark) int {
	switch {
	case m.Band > p.rings:
		return p.rowY[p.rings] + p.height[p.rings]
	case m.Band < -p.rings:
		return p.rowY[-p.rings] - 1
	}
	return p.rowY[m.Band] + m.Offset
}

// SlotBlock is the ground a slot covers, on the plan's rings or on the
// ring just past them, which has no column or row yet: a slot there is
// as wide as the smallest block, across the avenue from the outer ring.
// It reports false for a slot further out than that. Item 85.
func (p Plan) SlotBlock(s Slot) (Block, bool) {
	col, cols, ok := p.slotSpan(s.Col, p.colX, p.width)
	if !ok {
		return Block{}, false
	}
	row, rows, ok := p.slotSpan(s.Row, p.rowY, p.height)
	if !ok {
		return Block{}, false
	}
	return Block{Min: Cell{Col: col, Row: row}, Cols: cols, Rows: rows}, true
}

func (p Plan) slotSpan(i int, start, size map[int]int) (int, int, bool) {
	switch {
	case abs(i) <= p.rings:
		return start[i], size[i], true
	case i == p.rings+1:
		return start[p.rings] + size[p.rings] + 1, minSlot, true
	case i == -p.rings-1:
		return start[-p.rings] - 1 - minSlot, minSlot, true
	}
	return 0, 0, false
}

// SlotAt is the slot whose ground a cell lies on, out to the ring just
// past the plan's, or false for an avenue or anywhere further out.
func (p Plan) SlotAt(c Cell) (Slot, bool) {
	for row := -p.rings - 1; row <= p.rings+1; row++ {
		for col := -p.rings - 1; col <= p.rings+1; col++ {
			s := Slot{Col: col, Row: row}
			if b, ok := p.SlotBlock(s); ok && b.Contains(c) {
				return s, true
			}
		}
	}
	return Slot{}, false
}

// Street reports a street cell's joins, if the cell is street.
func (p Plan) Street(c Cell) (int, bool) {
	mask, ok := p.street[c]
	return mask, ok
}

func (p Plan) IsStreet(c Cell) bool {
	_, ok := p.street[c]
	return ok
}

// IsPark reports whether a cell is park: an unused block or the belt.
func (p Plan) IsPark(c Cell) bool {
	return p.park[c]
}

// SpanCols is the width of the rings kerb to kerb: what the storage
// block along the south spans.
func (p Plan) SpanCols() int {
	return p.span
}

// Rings is how many rings of blocks surround the plaza.
func (p Plan) Rings() int {
	return p.rings
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// plotCols and plotRows are how much plaza a civic building takes. The
// widest of them is two and a half cells across and the tallest two
// deep, so three by two holds any of them with a little room round the
// walls.
const (
	plotCols = 3
	plotRows = 2
)

// plazaPlots reserves a corner of the plaza for each civic building:
// the plant top-left, the hall top-right, the library bottom-left. The
// corners are where pkg/city has always put them; the difference is that
// the plan now says so, once, where the planting can see it.
func plazaPlots(plaza Block) Plots {
	if plaza.Cols < 2*plotCols || plaza.Rows < 2*plotRows {
		// Too small to hold three plots without them touching. Reserve
		// nothing rather than reserve overlapping ground — the planting
		// filling a cramped plaza is a smaller wrong than two buildings
		// on one plot.
		return Plots{}
	}
	right := plaza.Min.Col + plaza.Cols - plotCols
	bottom := plaza.Min.Row + plaza.Rows - plotRows
	return Plots{
		Plant:   Block{Min: plaza.Min, Cols: plotCols, Rows: plotRows},
		Hall:    Block{Min: Cell{Col: right, Row: plaza.Min.Row}, Cols: plotCols, Rows: plotRows},
		Library: Block{Min: Cell{Col: plaza.Min.Col, Row: bottom}, Cols: plotCols, Rows: plotRows},
	}
}
