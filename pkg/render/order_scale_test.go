package render

import (
	"container/heap"
	"fmt"
	"math/rand"
	"slices"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// orderBySwapping is order as it was, its first pass a stable sort that
// swapped whole drawables: the oracle the index sort must agree with.
func orderBySwapping(items []drawable) []drawable {
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

// named is drawables that say their names when drawn, and the order a
// list of them draws in.
type named struct {
	items []drawable
	said  *[]string
}

func newNamed(cam *city.Camera, rects []city.Rect, levels []int) named {
	said := &[]string{}
	n := named{said: said}
	for i, r := range rects {
		name := fmt.Sprint(i)
		d := footprintAt(cam, r, func() { *said = append(*said, name) })
		d.level = levels[i]
		n.items = append(n.items, d)
	}
	return n
}

func (n named) drawn(ordered []drawable) []string {
	*n.said = (*n.said)[:0]
	for _, d := range ordered {
		d.draw()
	}
	return slices.Clone(*n.said)
}

// Item 270: the depth sort's first pass swapped whole drawables through
// a reflective stable sort, three quarters of the sort's time at 3,000
// of them.
func TestOrderMatchesTheStableSort(t *testing.T) {
	for _, heading := range []int{0, 90, 180, 270} {
		t.Run(fmt.Sprintf("when the camera faces %d degrees", heading), func(t *testing.T) {
			cam := city.NewCamera()
			cam.Heading = heading

			t.Run("and 3,000 drawables with repeated depths, levels and overlapping footprints are ordered", func(t *testing.T) {
				r := rand.New(rand.NewSource(int64(270 + heading)))
				var rects []city.Rect
				var levels []int
				for range 3000 {
					x, y := float64(r.Intn(120))*city.Tile, float64(r.Intn(120))*city.Tile
					w, h := float64(1+r.Intn(4))*city.Tile, float64(1+r.Intn(4))*city.Tile
					rects = append(rects, city.RectAt(x, y, w, h))
					levels = append(levels, r.Intn(2))
				}
				n := newNamed(cam, rects, levels)
				want := n.drawn(orderBySwapping(slices.Clone(n.items)))
				got := n.drawn(order(cam, slices.Clone(n.items)))

				t.Run("it should put them in the order the struct-swapping stable sort did", func(t *testing.T) {
					assert.Equal(t, want, got)
				})
			})

			for _, flip := range []bool{false, true} {
				t.Run(fmt.Sprintf("and two drawables of equal depth that differ are ordered, given the other way round: %v", flip), func(t *testing.T) {
					rects := []city.Rect{city.RectAt(0, 4*city.Tile, city.Tile, city.Tile), city.RectAt(4*city.Tile, 0, city.Tile, city.Tile)}
					if flip {
						rects[0], rects[1] = rects[1], rects[0]
					}
					n := newNamed(cam, rects, []int{0, 0})
					want := n.drawn(orderBySwapping(slices.Clone(n.items)))
					got := n.drawn(order(cam, slices.Clone(n.items)))

					t.Run("it should keep them as the stable sort does", func(t *testing.T) {
						assert.Equal(t, want, got)
					})
				})
			}
		})
	}
}

// depthsOf is each drawable's footprint, in the order given: what tells
// two drawables of one depth apart.
func depthsOf(items []drawable) []city.Rect {
	out := make([]city.Rect, len(items))
	for i, d := range items {
		out[i] = d.turned
	}
	return out
}

// Item 270: the depth sort sorted every drawable afresh each frame,
// though from one frame to the next a few cars move and the rest stand.
// It starts from the last frame's order and repairs it, and falls back to
// sorting afresh when the repair runs past a budget.
func TestByDepthAcrossFrames(t *testing.T) {
	const n, movers = 3000, 40
	cam := city.NewCamera()
	frame := func(r *rand.Rand, at []city.Point) []drawable {
		items := make([]drawable, len(at))
		for i, p := range at {
			items[i] = footprintAt(cam, city.RectAt(p.X, p.Y, 2*city.Tile, 2*city.Tile), func() {})
		}
		return items
	}
	place := func(seed int64, count int) []city.Point {
		r := rand.New(rand.NewSource(seed))
		at := make([]city.Point, count)
		for i := range at {
			at[i] = city.Point{X: float64(r.Intn(160)) * city.Tile, Y: float64(r.Intn(160)) * city.Tile}
		}
		return at
	}

	t.Run("when 3,000 drawables are sorted frame after frame, forty of them moving a little each frame", func(t *testing.T) {
		r := rand.New(rand.NewSource(27))
		at := place(270, n)
		var s depthSorter
		same, repaired := true, 0
		for f := range 20 {
			for k := range movers {
				at[k].X += float64(r.Intn(3)-1) * city.Tile / 4
				at[k].Y += float64(r.Intn(3)-1) * city.Tile / 4
			}
			items := frame(r, at)
			before := s.moves
			got := s.byDepth(slices.Clone(items))
			if !slices.Equal(depthsOf(got), depthsOf(byDepth(nil, slices.Clone(items)))) {
				same = false
			}
			if f > 0 {
				repaired = max(repaired, s.moves-before)
			}
		}

		t.Run("it should give each frame the order a fresh sort gives", func(t *testing.T) {
			assert.True(t, same)
		})

		t.Run("it should repair each frame after the first in fewer moves than there are drawables", func(t *testing.T) {
			assert.Less(t, repaired, n)
		})

		t.Run("it should not fall back to sorting afresh", func(t *testing.T) {
			assert.Zero(t, s.fallbacks)
		})
	})

	t.Run("when drawables come and go, so the count changes", func(t *testing.T) {
		var s depthSorter
		s.byDepth(frame(nil, place(270, n)))
		items := frame(nil, place(271, n-17))
		got := s.byDepth(slices.Clone(items))

		t.Run("it should give the order a fresh sort gives", func(t *testing.T) {
			assert.Equal(t, depthsOf(byDepth(nil, slices.Clone(items))), depthsOf(got))
		})
	})

	for _, c := range []struct {
		name string
		next func(at []city.Point) []drawable
	}{
		{"when the camera turns, so every depth changes", func(at []city.Point) []drawable {
			cam.Turn(1, city.Point{X: 80 * city.Tile, Y: 80 * city.Tile})
			defer cam.Turn(-1, city.Point{X: 80 * city.Tile, Y: 80 * city.Tile})
			return frame(nil, at)
		}},
		{"when the last frame's order is the reverse of this one's", func(at []city.Point) []drawable {
			items := frame(nil, at)
			slices.Reverse(items)
			return items
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			at := place(270, n)
			var s depthSorter
			s.byDepth(frame(nil, at))
			items := c.next(at)
			before := s.moves
			got := s.byDepth(slices.Clone(items))
			moved := s.moves - before

			t.Run("it should give the order a fresh sort gives", func(t *testing.T) {
				assert.Equal(t, depthsOf(byDepth(nil, slices.Clone(items))), depthsOf(got))
			})

			t.Run("it should give up the repair within its budget and sort afresh", func(t *testing.T) {
				assert.Equal(t, [2]any{1, true}, [2]any{s.fallbacks, moved <= repairBudget*n})
			})
		})
	}
}

func BenchmarkOrderAtScale(b *testing.B) {
	cam := city.NewCamera()
	r := rand.New(rand.NewSource(270))
	var items []drawable
	for range 3000 {
		x, y := float64(r.Intn(120))*city.Tile, float64(r.Intn(120))*city.Tile
		items = append(items, footprintAt(cam, city.RectAt(x, y, 2*city.Tile, 2*city.Tile), func() {}))
	}
	buf := make([]drawable, len(items))
	b.ReportAllocs()
	for b.Loop() {
		copy(buf, items)
		order(cam, buf)
	}
}
