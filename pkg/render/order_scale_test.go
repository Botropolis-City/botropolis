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
