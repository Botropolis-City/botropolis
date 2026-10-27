package render

import (
	"cmp"
	"container/heap"
	"slices"

	"botropolis.city/botropolis/pkg/city"
)

// drawable is one thing to paint, carrying the footprint it stands on
// rather than a single depth number.
//
// Bug 42: a footprint spans a range of depths, and no single scalar
// orders a point against a box in this projection. depth is still the
// back-most corner and is the first pass, because it is nearly right
// and cheap; turned is what the pairwise predicate actually reads.
type drawable struct {
	depth  float64   // back-most corner in the camera's frame
	far    float64   // front-most corner, for pruning comparisons
	left   float64   // the screen columns the footprint covers, in world
	right  float64   // units — only the ordering of these matters
	turned city.Rect // the footprint in the camera's frame
	draw   func()
	// keyed, when set, notes it in place of note, keeping its record
	// while what it is drawn from holds (item 212).
	keyed func()
	// runs is whether its keyed draw must run every frame, as it gives
	// back what the frame goes on to use, so a frame holding the still
	// city does not note it as the last did (item 212).
	runs bool
	// kept is whether it stays at full strength while a hovered car
	// fades everything not tied to it (item 108).
	kept bool
	// level is how far off the ground it stands: the bridge's deck and
	// what drives on it (item 132).
	level int
	// moves is whether it is drawn somewhere new each frame, so it is
	// drawn over the standing city rather than composed into it (item
	// 193).
	moves bool
}

// footprint is a drawable standing on a world rectangle.
func footprintAt(cam *city.Camera, r city.Rect, draw func()) drawable {
	t := cam.TurnRect(r)
	return drawable{
		depth:  t.Min.X + t.Min.Y,
		far:    t.Max.X + t.Max.Y,
		left:   t.Min.X - t.Max.Y,
		right:  t.Max.X - t.Min.Y,
		turned: t,
		draw:   draw,
	}
}

// standingAt is a drawable on a world point, promoted to a cell-sized
// box so there is one rule for points and footprints alike.
func standingAt(cam *city.Camera, p city.Point, draw func()) drawable {
	half := city.Tile / 2
	return footprintAt(cam, city.Rect{
		Min: city.Point{X: p.X - half, Y: p.Y - half},
		Max: city.Point{X: p.X + half, Y: p.Y + half},
	}, draw)
}

// order puts the drawables back to front.
//
// The depth sort is the first pass and the pairwise predicate corrects
// it. Only pairs that could hide one another are compared: their depth
// intervals have to overlap, and so do the screen columns their
// footprints cover.
//
// That second test is not an optimisation. Without it the predicate
// cycles — a box west of another and north of it is "behind" it on the
// x axis while the other is "behind" it on the y axis, so each must be
// drawn first — and two things that share no screen column cannot
// occlude each other anyway, so there is nothing to decide.
//
// A cycle that survives anyway costs nothing worse than the old
// behaviour: whatever cannot be ordered is appended in depth order
// rather than dropped.
func order(cam *city.Camera, items []drawable) []drawable { return orderWith(nil, cam, items) }

// orderWith is order, its depth pass by way of s when there is one.
func orderWith(s *depthSorter, cam *city.Camera, items []drawable) []drawable {
	n := len(items)
	if n < 2 {
		return items
	}
	items = byDepth(s, items)
	var edges [][2]int32
	indegree := make([]int32, n)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n && items[j].depth <= items[i].far; j++ {
			if items[i].right <= items[j].left || items[j].right <= items[i].left {
				continue
			}
			// Item 132: of two things on one patch of ground, the one
			// standing higher draws after — a train under the deck, a
			// truck on it.
			switch {
			case items[i].level != items[j].level && overlapsTurned(items[i].turned, items[j].turned):
				lo, hi := i, j
				if items[j].level < items[i].level {
					lo, hi = j, i
				}
				edges = append(edges, [2]int32{int32(lo), int32(hi)})
				indegree[hi]++
			case city.BehindTurned(items[i].turned, items[j].turned):
				edges = append(edges, [2]int32{int32(i), int32(j)})
				indegree[j]++
			case city.BehindTurned(items[j].turned, items[i].turned):
				edges = append(edges, [2]int32{int32(j), int32(i)})
				indegree[i]++
			}
		}
	}
	// Each drawable's successors, filed together in one slice: start[i]
	// to start[i+1] in next are what must draw after i.
	start := make([]int32, n+1)
	for _, e := range edges {
		start[e[0]+1]++
	}
	for i := 1; i <= n; i++ {
		start[i] += start[i-1]
	}
	next := make([]int32, len(edges))
	fill := slices.Clone(start[:n])
	for _, e := range edges {
		next[fill[e[0]]] = e[1]
		fill[e[0]]++
	}
	var ready readyQueue
	for i := 0; i < n; i++ {
		if indegree[i] == 0 {
			ready = append(ready, int32(i))
		}
	}
	heap.Init(&ready)
	out := make([]drawable, 0, n)
	drawn := make([]bool, n)
	for len(ready) > 0 {
		i := ready.pop()
		out = append(out, items[i])
		drawn[i] = true
		for _, j := range next[start[i]:start[i+1]] {
			indegree[j]--
			if indegree[j] == 0 {
				ready.push(j)
			}
		}
	}
	for i := 0; i < n; i++ {
		if !drawn[i] {
			out = append(out, items[i])
		}
	}
	return out
}

// overlapsTurned is whether two footprints share ground, not merely an
// edge.
func overlapsTurned(a, b city.Rect) bool {
	return a.Min.X < b.Max.X && b.Min.X < a.Max.X && a.Min.Y < b.Max.Y && b.Min.Y < a.Max.Y
}

// readyQueue pops the lowest index first, so a drawable only moves when
// the predicate says it must and the depth order survives everywhere
// else.
type readyQueue []int32

func (q readyQueue) Len() int           { return len(q) }
func (q readyQueue) Less(a, b int) bool { return q[a] < q[b] }
func (q readyQueue) Swap(a, b int)      { q[a], q[b] = q[b], q[a] }
func (q *readyQueue) Push(x any)        { *q = append(*q, x.(int32)) }
func (q *readyQueue) Pop() any          { old := *q; n := len(old); x := old[n-1]; *q = old[:n-1]; return x }

// byDepth is items back to front by depth, those of equal depth in the
// order they came: their positions sorted by depth and then by position,
// a total order, and the drawables moved once. A stable sort that
// swapped whole drawables was three quarters of the depth sort at 3,000
// of them (item 270).
func byDepth(s *depthSorter, items []drawable) []drawable {
	var at []int32
	if s != nil {
		at = s.sort(items)
	} else {
		at = make([]int32, len(items))
		for i := range at {
			at[i] = int32(i)
		}
		slices.SortFunc(at, func(a, b int32) int { return deeper(items, a, b) })
	}
	sorted := make([]drawable, len(items))
	for i, j := range at {
		sorted[i] = items[j]
	}
	return sorted
}

// push and pop are heap.Push and heap.Pop without boxing each index in
// an interface, which allocated for every drawable a frame (item 270).
func (q *readyQueue) push(x int32) {
	*q = append(*q, x)
	h := *q
	for i := len(h) - 1; i > 0; {
		parent := (i - 1) / 2
		if h[parent] <= h[i] {
			break
		}
		h[parent], h[i] = h[i], h[parent]
		i = parent
	}
}

func (q *readyQueue) pop() int32 {
	h := *q
	top := h[0]
	last := len(h) - 1
	h[0] = h[last]
	h = h[:last]
	for i := 0; ; {
		least, l, r := i, 2*i+1, 2*i+2
		if l < len(h) && h[l] < h[least] {
			least = l
		}
		if r < len(h) && h[r] < h[least] {
			least = r
		}
		if least == i {
			break
		}
		h[i], h[least] = h[least], h[i]
		i = least
	}
	*q = h
	return top
}

// deeper orders two positions in items by depth, and those of one depth
// by position: a total order, so every sort of it gives one answer.
func deeper(items []drawable, a, b int32) int {
	if c := cmp.Compare(items[a].depth, items[b].depth); c != 0 {
		return c
	}
	return cmp.Compare(a, b)
}

// repairBudget is how many moves a drawable the repair of the last
// frame's order may take, on average, before it gives up and sorts
// afresh: a turn or a new snapshot reorders nearly everything, and
// repairing that by insertion would take about n²/2 moves.
const repairBudget = 4

// depthSorter keeps the frame's depth order, so the next frame, in which
// a few cars have moved and the rest stand where they stood, repairs it
// rather than sorting every drawable afresh (item 270). Whatever it
// starts from, the order it gives is the one sorting afresh gives,
// since deeper is a total order.
type depthSorter struct {
	last []int32
	// moves is how many places the repairs have moved positions, and
	// fallbacks how many times a repair gave up and sorted afresh.
	moves, fallbacks int
}

// byDepth is items in depth order, by way of the sorter.
func (s *depthSorter) byDepth(items []drawable) []drawable { return byDepth(s, items) }

// sort is the positions of items in depth order: last frame's order
// repaired by insertion when the count holds, otherwise, or when the
// repair runs past its budget, sorted afresh.
func (s *depthSorter) sort(items []drawable) []int32 {
	n := len(items)
	if len(s.last) != n {
		s.last = make([]int32, n)
		for i := range s.last {
			s.last[i] = int32(i)
		}
		slices.SortFunc(s.last, func(a, b int32) int { return deeper(items, a, b) })
		return s.last
	}
	at, budget := s.last, repairBudget*n
	for i := 1; i < n; i++ {
		x := at[i]
		j := i
		for j > 0 && deeper(items, at[j-1], x) > 0 {
			at[j] = at[j-1]
			j--
			s.moves++
			if budget--; budget == 0 {
				s.fallbacks++
				for k := range at {
					at[k] = int32(k)
				}
				slices.SortFunc(at, func(a, b int32) int { return deeper(items, a, b) })
				return at
			}
		}
		at[j] = x
	}
	return at
}
