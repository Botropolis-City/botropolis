package render

import (
	"image"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
)

// Item 193: what is drawn again inside the rectangles something moves
// in is played as one stream, a command at a time into every rectangle
// it meets, so the same draw into several rectangles is one run of
// draws rather than one per rectangle, and each rectangle still gets
// its draws in the frame's order.
func TestDirtyStream(t *testing.T) {
	left, right := image.Rect(0, 0, 10, 10), image.Rect(20, 0, 30, 10)
	rects := []image.Rectangle{left, right}
	both := image.Rect(0, 0, 30, 10)
	for _, c := range []struct {
		name    string
		entries []entry
		cmds    []image.Rectangle
		want    []streamPlay
	}{
		{
			"when two rectangles share a drawable",
			[]entry{{inked: inked{from: 0, to: 2, box: both}}},
			[]image.Rectangle{both, both},
			[]streamPlay{{0, 0}, {0, 1}, {1, 0}, {1, 1}},
		},
		{
			"when drawables meet different rectangles",
			[]entry{{inked: inked{from: 0, to: 1, box: right}}, {inked: inked{from: 1, to: 2, box: left}}},
			[]image.Rectangle{right, left},
			[]streamPlay{{0, 1}, {1, 0}},
		},
		{
			"when one of a drawable's draws misses a rectangle it meets",
			[]entry{{inked: inked{from: 0, to: 2, box: both}}},
			[]image.Rectangle{left, both},
			[]streamPlay{{0, 0}, {1, 0}, {1, 1}},
		},
		{
			"when a drawable meets no rectangle",
			[]entry{{inked: inked{from: 0, to: 1, box: image.Rect(50, 50, 60, 60)}}},
			[]image.Rectangle{image.Rect(50, 50, 60, 60)},
			nil,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Run("it should play each draw into each rectangle it meets, a draw at a time, in the frame's order", func(t *testing.T) {
				assert.Equal(t, c.want, stream(nil, c.entries, func(i int) image.Rectangle { return c.cmds[i] }, rects))
			})
		})
	}
}

// Item 193: draws into one rectangle that do not overlap touch different
// pixels, so their order is free, and draws from one source are put
// together so they go down as one; draws that overlap keep their order.
func TestDirtyBatch(t *testing.T) {
	a, b := image.Rect(0, 0, 4, 4), image.Rect(6, 0, 10, 4)
	for _, c := range []struct {
		name  string
		plays []streamPlay
		boxes []image.Rectangle
		srcs  []int
		want  []streamPlay
	}{
		{
			"when draws from two sources alternate and none overlap",
			[]streamPlay{{0, 0}, {1, 0}, {2, 0}, {3, 0}},
			[]image.Rectangle{a, b, a.Add(image.Pt(0, 5)), b.Add(image.Pt(0, 5))},
			[]int{1, 2, 1, 2},
			[]streamPlay{{0, 0}, {2, 0}, {1, 0}, {3, 0}},
		},
		{
			"when a draw overlaps one from another source before it",
			[]streamPlay{{0, 0}, {1, 0}, {2, 0}},
			[]image.Rectangle{a, a, b},
			[]int{1, 2, 1},
			[]streamPlay{{0, 0}, {2, 0}, {1, 0}},
		},
		{
			"when a draw overlaps the draw it would be moved ahead of",
			[]streamPlay{{0, 0}, {1, 0}, {2, 0}},
			[]image.Rectangle{a, a, a},
			[]int{1, 2, 1},
			[]streamPlay{{0, 0}, {1, 0}, {2, 0}},
		},
		{
			"when overlapping draws are in different rectangles",
			[]streamPlay{{0, 0}, {1, 1}, {2, 0}},
			[]image.Rectangle{a, a, a},
			[]int{1, 2, 1},
			[]streamPlay{{0, 0}, {2, 0}, {1, 1}},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := batch(nil, c.plays, func(i int) image.Rectangle { return c.boxes[i] }, func(i int) int { return c.srcs[i] })

			t.Run("it should put draws from one source together without passing a draw they overlap", func(t *testing.T) {
				assert.Equal(t, c.want, got)
			})
		})
	}
}

// Item 161: a draw's page was found by reflecting on Ebitengine's
// private field for the image a sub-image was cut from, on every draw
// of every frame. The page is now kept beside the draw by whoever cut
// the sub-image. The reflection stays here only, as the reference: two
// draws must share a batch key exactly when they read the same page.
func TestRecordedDrawsKeyByTheirPage(t *testing.T) {
	cutFrom := func(img *ebiten.Image) uintptr {
		if img == nil {
			return 0
		}
		if f := reflect.ValueOf(img).Elem().FieldByName("original"); f.IsValid() && !f.IsNil() {
			return f.Pointer()
		}
		return uintptr(unsafe.Pointer(img))
	}
	day := time.Now()
	for _, c := range []struct {
		name string
		open func(t *testing.T) *Drive
	}{
		{"the metropolis by day", func(t *testing.T) *Drive {
			d, err := NewDrive(DriveOptions{Home: stageMetropolis(t, day), Now: atHour(day, 14)})
			require.NoError(t, err)
			require.NoError(t, d.Run(60))
			return d
		}},
		{"the metropolis by night", func(t *testing.T) *Drive {
			d, err := NewDrive(DriveOptions{Home: stageMetropolis(t, day), Now: atHour(day, 22)})
			require.NoError(t, err)
			require.NoError(t, d.Run(60))
			return d
		}},
		{"the metropolis with a view up", func(t *testing.T) *Drive {
			d, err := NewDrive(DriveOptions{Home: stageMetropolis(t, day), Now: atHour(day, 14)})
			require.NoError(t, err)
			d.game.scene.SetView(city.ViewSpend)
			require.NoError(t, d.Run(60))
			return d
		}},
		{"217 sessions at full zoom", func(t *testing.T) *Drive { return fullZoom(t, DriveOptions{}) }},
	} {
		t.Run("when "+c.name+" is recorded", func(t *testing.T) {
			l := c.open(t).game.rec
			keys := map[uintptr]map[uintptr]bool{}
			pages := map[uintptr]map[uintptr]bool{}
			for i := range l.cmds {
				switch l.cmds[i].kind {
				case cmdImage, cmdColorM, cmdMesh:
				default:
					continue
				}
				k, p := l.pageKey(i), cutFrom(l.cmds[i].src)
				if keys[k] == nil {
					keys[k] = map[uintptr]bool{}
				}
				if pages[p] == nil {
					pages[p] = map[uintptr]bool{}
				}
				keys[k][p], pages[p][k] = true, true
			}
			var split, merged int
			for _, ps := range keys {
				if len(ps) > 1 {
					merged++
				}
			}
			for _, ks := range pages {
				if len(ks) > 1 {
					split++
				}
			}
			require.NotEmpty(t, keys, "nothing was recorded")

			t.Run("it should key apart no two draws that read one page, nor key together two that read two", func(t *testing.T) {
				assert.Equal(t, [2]int{0, 0}, [2]int{split, merged})
			})
		})
	}
}

// Item 161: ordering a frame's redrawn draws asked for each draw's
// source again on every comparison, and each ask went through a map; at
// fit on the real history it was more than half of laying the standing
// city.
func TestDirtyBatchAsksOnce(t *testing.T) {
	const n = 60
	plays := make([]streamPlay, n)
	for i := range plays {
		plays[i] = streamPlay{cmd: i, rect: i % 3}
	}
	boxAsks, srcAsks := map[int]int{}, map[int]int{}
	batch(nil, plays,
		func(i int) image.Rectangle { boxAsks[i]++; return image.Rect(i, 0, i+1, 1) },
		func(i int) int { srcAsks[i]++; return i % 4 })
	most := func(asks map[int]int) int {
		m := 0
		for _, k := range asks {
			m = max(m, k)
		}
		return m
	}

	t.Run("when draws in several rectangles are batched", func(t *testing.T) {
		t.Run("it should ask for each draw's source once", func(t *testing.T) {
			assert.Equal(t, 1, most(srcAsks))
		})

		t.Run("it should ask for each draw's box once", func(t *testing.T) {
			assert.Equal(t, 1, most(boxAsks))
		})
	})
}
