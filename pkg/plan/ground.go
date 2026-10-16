package plan

import "math"

// The trees that mean nothing are ground, and are planted so that they
// look it: clustered where a smooth noise rises over a threshold and
// bare where it falls under it, and spaced by Poisson-disk sampling —
// a candidate in every third of a cell, each nudged off its centre and
// kept only if no tree already stands within TreeSpacing. They grow on
// the parks, the belt and the corridors, so the wood runs between the
// blocks; the city clears it off the corridors a road is laid on. Every
// choice is seeded from position, so a plan always grows the same wood.
// Item 85.

// TreeSpacing is the least distance between two ground trees, in cells.
const TreeSpacing = 0.9

const (
	// treeFeature is how many cells one rise of the mask spans, and
	// treeMask the level under which the ground is left bare.
	treeFeature = 3.0
	treeMask    = 0.05
	treeSubs    = 3
)

func (p *Plan) plantGround(onRails map[Cell]bool) {
	offered := map[Cell][]int{}
	for _, b := range p.Parks {
		g := grove(b)
		eachCell(b, func(c Cell) { offered[c] = g })
	}
	all := everySpecies()
	for _, b := range p.Belt {
		eachCell(b, func(c Cell) { offered[c] = all })
	}
	for c := range p.street {
		offered[c] = all
	}
	for c := range offered {
		if onRails[c] || p.Plaza.Contains(c) {
			delete(offered, c)
		}
	}
	for _, b := range p.Blocks {
		eachCell(b, func(c Cell) { delete(offered, c) })
	}
	for _, r := range p.River {
		delete(offered, r.Cell)
	}
	type bucket struct{ x, y int }
	near := map[bucket][][2]float64{}
	free := func(x, y float64) bool {
		bx, by := int(math.Floor(x/TreeSpacing)), int(math.Floor(y/TreeSpacing))
		for dx := -1; dx <= 1; dx++ {
			for dy := -1; dy <= 1; dy++ {
				for _, q := range near[bucket{bx + dx, by + dy}] {
					if math.Hypot(q[0]-x, q[1]-y) < TreeSpacing {
						return false
					}
				}
			}
		}
		return true
	}
	for row := p.Bounds.Min.Row; row < p.Bounds.Min.Row+p.Bounds.Rows; row++ {
		for sj := 0; sj < treeSubs; sj++ {
			for col := p.Bounds.Min.Col; col < p.Bounds.Min.Col+p.Bounds.Cols; col++ {
				species := offered[Cell{col, row}]
				if species == nil {
					continue
				}
				for si := 0; si < treeSubs; si++ {
					seed := seedOf(col, row, si, sj)
					nudge := 0.4 / treeSubs
					x := float64(col) + (float64(si)+0.5)/treeSubs + unitOf(seed)*nudge
					y := float64(row) + (float64(sj)+0.5)/treeSubs + unitOf(seed>>16)*nudge
					if Noise(x/treeFeature, y/treeFeature) < treeMask || !free(x, y) {
						continue
					}
					b := bucket{int(math.Floor(x / TreeSpacing)), int(math.Floor(y / TreeSpacing))}
					near[b] = append(near[b], [2]float64{x, y})
					p.Trees = append(p.Trees, Tree{Cell: Cell{col, row}, Kind: ParkTree,
						Variant: species[int(seed>>8)%len(species)], DX: x - float64(col) - 0.5, DY: y - float64(row) - 0.5})
				}
			}
		}
	}
}

func eachCell(b Block, f func(Cell)) {
	for row := b.Min.Row; row < b.Min.Row+b.Rows; row++ {
		for col := b.Min.Col; col < b.Min.Col+b.Cols; col++ {
			f(Cell{col, row})
		}
	}
}

func hash2(x, y int) float64 {
	h := uint32(x)*374761393 + uint32(y)*668265263
	h = (h ^ (h >> 13)) * 1274126177
	h ^= h >> 16
	return float64(h&0xffff)/0xffff*2 - 1
}

// Noise is smooth value noise in [-1, 1] over the plane, seeded only by
// position, so anything drawn from it is the same every run.
func Noise(x, y float64) float64 {
	x0, y0 := math.Floor(x), math.Floor(y)
	fx, fy := x-x0, y-y0
	sx, sy := fx*fx*(3-2*fx), fy*fy*(3-2*fy)
	ix, iy := int(x0), int(y0)
	a := hash2(ix, iy) + (hash2(ix+1, iy)-hash2(ix, iy))*sx
	b := hash2(ix, iy+1) + (hash2(ix+1, iy+1)-hash2(ix, iy+1))*sx
	return a + (b-a)*sy
}
