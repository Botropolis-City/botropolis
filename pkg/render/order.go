package render

import (
	"container/heap"
	"sort"

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
func order(cam *city.Camera, items []drawable) []drawable {
	sort.SliceStable(items, func(i, j int) bool { return items[i].depth < items[j].depth })
	n := len(items)
	if n < 2 {
		return items
	}
	after := make([][]int32, n)
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
				after[lo] = append(after[lo], int32(hi))
				indegree[hi]++
			case city.BehindTurned(items[i].turned, items[j].turned):
				after[i] = append(after[i], int32(j))
				indegree[j]++
			case city.BehindTurned(items[j].turned, items[i].turned):
				after[j] = append(after[j], int32(i))
				indegree[i]++
			}
		}
	}
	ready := &readyQueue{}
	for i := 0; i < n; i++ {
		if indegree[i] == 0 {
			*ready = append(*ready, int32(i))
		}
	}
	heap.Init(ready)
	out := make([]drawable, 0, n)
	drawn := make([]bool, n)
	for ready.Len() > 0 {
		i := heap.Pop(ready).(int32)
		out = append(out, items[i])
		drawn[i] = true
		for _, j := range after[i] {
			indegree[j]--
			if indegree[j] == 0 {
				heap.Push(ready, j)
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
