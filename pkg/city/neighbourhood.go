package city

import (
	"math"
	"sort"

	"botropolis.city/botropolis/pkg/state"
)

// A Neighbourhood is a group's buildings standing on adjacent lots in
// one district: one block of ground, so the group is known by its place
// before any line or badge is read (item 348).
type Neighbourhood struct {
	Group     *state.Group
	Buildings []*Building
	// Ground is the lots and the gaps between adjacent ones, paved as
	// one block.
	Ground []Rect
	// Gate is the block's corner nearest the district's driveway, where
	// its marker stands.
	Gate Point
}

// Neighbourhoods are a district's neighbourhoods: each run of a group's
// buildings joined lot to lot, of two or more. A building alone, or in
// no group, is in none.
func (c *City) Neighbourhoods(d *District) []Neighbourhood {
	if d.Depot || d.columns == 0 {
		return nil
	}
	at := map[int]*Building{}
	for _, b := range d.Buildings {
		at[b.slot] = b
	}
	same := func(a, b *Building) bool { return a != nil && b != nil && a.Group != nil && a.Group == b.Group }
	neighbours := func(b *Building) []*Building {
		col := b.slot % d.columns
		var out []*Building
		for _, n := range []struct {
			slot int
			ok   bool
		}{{b.slot - 1, col > 0}, {b.slot + 1, col < d.columns-1}, {b.slot - d.columns, true}, {b.slot + d.columns, true}} {
			if o := at[n.slot]; n.ok && same(b, o) {
				out = append(out, o)
			}
		}
		return out
	}
	seen := map[*Building]bool{}
	var hoods []Neighbourhood
	for _, b := range d.Buildings {
		if seen[b] || b.Group == nil || len(neighbours(b)) == 0 {
			continue
		}
		run := []*Building{b}
		seen[b] = true
		for i := 0; i < len(run); i++ {
			for _, o := range neighbours(run[i]) {
				if !seen[o] {
					seen[o] = true
					run = append(run, o)
				}
			}
		}
		sort.Slice(run, func(i, j int) bool { return run[i].slot < run[j].slot })
		hoods = append(hoods, c.neighbourhood(d, run, at))
	}
	return hoods
}

// neighbourhood paves a run's lots and the gaps between them, and finds
// its gate.
func (c *City) neighbourhood(d *District, run []*Building, at map[int]*Building) Neighbourhood {
	n := Neighbourhood{Group: run[0].Group, Buildings: run}
	in := map[int]bool{}
	for _, b := range run {
		in[b.slot] = true
	}
	for _, b := range run {
		r := b.Rect
		n.Ground = append(n.Ground, r)
		col := b.slot % d.columns
		right, below := in[b.slot+1] && col < d.columns-1, in[b.slot+d.columns]
		if right {
			n.Ground = append(n.Ground, Rect{Min: Point{X: r.Max.X, Y: r.Min.Y}, Max: Point{X: at[b.slot+1].Rect.Min.X, Y: r.Max.Y}})
		}
		if below {
			n.Ground = append(n.Ground, Rect{Min: Point{X: r.Min.X, Y: r.Max.Y}, Max: Point{X: r.Max.X, Y: at[b.slot+d.columns].Rect.Min.Y}})
		}
		if right && below && in[b.slot+d.columns+1] {
			n.Ground = append(n.Ground, Rect{Min: r.Max, Max: at[b.slot+d.columns+1].Rect.Min})
		}
	}
	toward := d.Rect.Center()
	if dw, ok := c.Driveway(d); ok {
		toward = dw.Lot
	}
	best := math.Inf(1)
	for _, b := range run {
		r := b.Rect
		for _, p := range []Point{r.Min, {X: r.Max.X, Y: r.Min.Y}, {X: r.Min.X, Y: r.Max.Y}, r.Max} {
			if dist := math.Hypot(p.X-toward.X, p.Y-toward.Y); dist < best {
				best, n.Gate = dist, p
			}
		}
	}
	return n
}

// Edges are the outline of a neighbourhood's ground: every side of it
// that faces outward, and none where two of its pieces meet, cut at
// each corner a piece makes.
func (n Neighbourhood) Edges() [][2]Point {
	var xs, ys []float64
	for _, r := range n.Ground {
		xs = append(xs, r.Min.X, r.Max.X)
		ys = append(ys, r.Min.Y, r.Max.Y)
	}
	xs, ys = uniqueSorted(xs), uniqueSorted(ys)
	filled := func(i, j int) bool {
		if i < 0 || j < 0 || i >= len(xs)-1 || j >= len(ys)-1 {
			return false
		}
		p := Point{X: (xs[i] + xs[i+1]) / 2, Y: (ys[j] + ys[j+1]) / 2}
		for _, r := range n.Ground {
			if p.X > r.Min.X && p.X < r.Max.X && p.Y > r.Min.Y && p.Y < r.Max.Y {
				return true
			}
		}
		return false
	}
	var edges [][2]Point
	for i := 0; i < len(xs)-1; i++ {
		for j := 0; j < len(ys)-1; j++ {
			if !filled(i, j) {
				continue
			}
			if !filled(i, j-1) {
				edges = append(edges, [2]Point{{X: xs[i], Y: ys[j]}, {X: xs[i+1], Y: ys[j]}})
			}
			if !filled(i, j+1) {
				edges = append(edges, [2]Point{{X: xs[i], Y: ys[j+1]}, {X: xs[i+1], Y: ys[j+1]}})
			}
			if !filled(i-1, j) {
				edges = append(edges, [2]Point{{X: xs[i], Y: ys[j]}, {X: xs[i], Y: ys[j+1]}})
			}
			if !filled(i+1, j) {
				edges = append(edges, [2]Point{{X: xs[i+1], Y: ys[j]}, {X: xs[i+1], Y: ys[j+1]}})
			}
		}
	}
	return edges
}

func uniqueSorted(vs []float64) []float64 {
	sort.Float64s(vs)
	out := vs[:0]
	for i, v := range vs {
		if i == 0 || v != out[len(out)-1] {
			out = append(out, v)
		}
	}
	return out
}

// Kerbs is how a neighbourhood's kerb is drawn: a low wall raised on
// the ground, the mark Aria chose of the three tried (2026-11-06, item
// 348), the same drawn over the city so the buildings in front do not
// hide it, still on trial, or none, for comparison frames.
type Kerbs int

const (
	KerbsRaised Kerbs = iota
	KerbsOnTop
	KerbsOff
)

// SetKerbs sets how neighbourhoods' kerbs are drawn.
func (s *Scene) SetKerbs(k Kerbs) { s.kerbs = k }

// Kerbs is how neighbourhoods' kerbs are drawn.
func (s *Scene) Kerbs() Kerbs { return s.kerbs }
