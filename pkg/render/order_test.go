package render

import (
	"fmt"
	"sort"
	"testing"

	"botropolis.city/botropolis/pkg/city"
	"github.com/stretchr/testify/assert"
)

// Bug 42, on the shape Aria saw: a conifer painted across a four-storey
// facade. The building spans eight cells of depth and was keyed at its
// back corner, so a tree standing behind it keyed deeper and drew over
// it.

func facade() city.Rect {
	return city.Rect{
		Min: city.Point{X: 5 * city.Tile, Y: 10 * city.Tile},
		Max: city.Point{X: 9 * city.Tile, Y: 14 * city.Tile},
	}
}

func namedOrder(cam *city.Camera, build func(record func(string) func()) []drawable) []string {
	var got []string
	record := func(name string) func() { return func() { got = append(got, name) } }
	for _, item := range order(cam, build(record)) {
		item.draw()
	}
	return got
}

func TestOrderPutsTreesBehindTheirBuilding(t *testing.T) {
	cam := city.NewCamera()

	t.Run("when a tree stands on the cell behind a building's back edge", func(t *testing.T) {
		got := namedOrder(cam, func(rec func(string) func()) []drawable {
			return []drawable{
				footprintAt(cam, facade(), rec("building")),
				standingAt(cam, city.Point{X: 7 * city.Tile, Y: 9 * city.Tile}, rec("tree")),
			}
		})

		t.Run("it should be painted before the building, not across its facade", func(t *testing.T) {
			assert.Equal(t, []string{"tree", "building"}, got)
		})
	})

	t.Run("when a tree stands off the building's near side", func(t *testing.T) {
		got := namedOrder(cam, func(rec func(string) func()) []drawable {
			return []drawable{
				footprintAt(cam, facade(), rec("building")),
				standingAt(cam, city.Point{X: 7 * city.Tile, Y: 15 * city.Tile}, rec("tree")),
			}
		})

		t.Run("it should be painted after the building, which the front-corner key got wrong", func(t *testing.T) {
			assert.Equal(t, []string{"building", "tree"}, got)
		})
	})
}

func TestOrderHoldsAtEveryHeading(t *testing.T) {
	// A depth bug that only shows at one heading is bug 25's family, so
	// all four are checked. These four were derived from the turned
	// boxes rather than guessed — the first guess had 90 and 270 the
	// wrong way round. Turning the camera a quarter moves the tree from
	// behind the building to beside and in front of it, and another
	// quarter puts it back behind.
	want := map[int][]string{
		0:   {"tree", "building"},
		90:  {"building", "tree"},
		180: {"building", "tree"},
		270: {"tree", "building"},
	}
	for _, heading := range []int{0, 90, 180, 270} {
		t.Run(fmt.Sprintf("when the camera faces %d", heading), func(t *testing.T) {
			cam := city.NewCamera()
			cam.Heading = heading
			got := namedOrder(cam, func(rec func(string) func()) []drawable {
				return []drawable{
					footprintAt(cam, facade(), rec("building")),
					standingAt(cam, city.Point{X: 7 * city.Tile, Y: 9 * city.Tile}, rec("tree")),
				}
			})

			t.Run("it should paint the pair the way the eye sees them", func(t *testing.T) {
				assert.Equal(t, want[heading], got)
			})
		})
	}
}

func TestOrderDrawsEverythingOnce(t *testing.T) {
	cam := city.NewCamera()

	t.Run("when the city is full of footprints and points", func(t *testing.T) {
		var items []drawable
		var want int
		for x := 0; x < 12; x++ {
			for y := 0; y < 12; y++ {
				at := city.Point{X: float64(x) * city.Tile * 2, Y: float64(y) * city.Tile * 2}
				items = append(items, standingAt(cam, at, func() {}))
				want++
			}
		}
		items = append(items, footprintAt(cam, facade(), func() {}))
		want++

		t.Run("it should paint every one of them, cycles or not", func(t *testing.T) {
			assert.Len(t, order(cam, items), want)
		})
	})
}

// BenchmarkOrder is the frame cost bug 42 asked for. The city it builds
// is the shape of a real one: a few hundred point-keyed things (trees,
// lamps, cars) scattered over a grid with a handful of multi-cell
// footprints among them.
func BenchmarkOrder(b *testing.B) {
	cam := city.NewCamera()
	var items []drawable
	for x := 0; x < 22; x++ {
		for y := 0; y < 22; y++ {
			at := city.Point{X: float64(x) * city.Tile * 1.5, Y: float64(y) * city.Tile * 1.5}
			items = append(items, standingAt(cam, at, func() {}))
		}
	}
	for i := 0; i < 24; i++ {
		x := float64(i%6) * city.Tile * 8
		y := float64(i/6) * city.Tile * 8
		items = append(items, footprintAt(cam, city.RectAt(x, y, 4*city.Tile, 4*city.Tile), func() {}))
	}
	scratch := make([]drawable, len(items))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copy(scratch, items)
		order(cam, scratch)
	}
}

// BenchmarkOrderDepthOnly is what order replaced: the single-key sort
// that could not tell a point from a box.
func BenchmarkOrderDepthOnly(b *testing.B) {
	cam := city.NewCamera()
	var items []drawable
	for x := 0; x < 22; x++ {
		for y := 0; y < 22; y++ {
			at := city.Point{X: float64(x) * city.Tile * 1.5, Y: float64(y) * city.Tile * 1.5}
			items = append(items, standingAt(cam, at, func() {}))
		}
	}
	for i := 0; i < 24; i++ {
		x := float64(i%6) * city.Tile * 8
		y := float64(i/6) * city.Tile * 8
		items = append(items, footprintAt(cam, city.RectAt(x, y, 4*city.Tile, 4*city.Tile), func() {}))
	}
	scratch := make([]drawable, len(items))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copy(scratch, items)
		sort.SliceStable(scratch, func(a, c int) bool { return scratch[a].depth < scratch[c].depth })
	}
}
