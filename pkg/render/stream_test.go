package render

import (
	"fmt"
	"image"
	"reflect"
	"runtime/debug"
	"testing"
	"time"
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/colorm"
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

// Item 236: Ebitengine draws a shrunk sprite from a mipmap level of its
// page, and each level is a texture of its own, so draws from one page
// at two levels cannot share a batch. The level is Ebitengine's own,
// worked out as it works it out.
func TestTheMipmapLevelADrawSamples(t *testing.T) {
	page := image.Pt(1024, 1024)
	scaled := func(sx, sy float64) ebiten.GeoM {
		var g ebiten.GeoM
		g.Scale(sx, sy)
		return g
	}
	t.Run("when an image draw is recorded", func(t *testing.T) {
		for _, c := range []struct {
			name   string
			size   int
			page   image.Point
			geo    ebiten.GeoM
			filter ebiten.Filter
			nomip  bool
			want   int
		}{
			{"and it is drawn at full size", 64, page, scaled(1, 1), ebiten.FilterLinear, false, 0},
			{"and it is filtered nearest and shrunk to a quarter", 64, page, scaled(0.25, 0.25), ebiten.FilterNearest, false, 0},
			{"and its mipmaps are disabled and it is shrunk to a quarter", 64, page, scaled(0.25, 0.25), ebiten.FilterLinear, true, 0},
			{"and it is shrunk to exactly half", 64, page, scaled(0.5, 0.5), ebiten.FilterLinear, false, 0},
			{"and it is shrunk to 0.45", 64, page, scaled(0.45, 0.45), ebiten.FilterLinear, false, 1},
			{"and it is shrunk to 0.2", 64, page, scaled(0.2, 0.2), ebiten.FilterLinear, false, 2},
			{"and it is shrunk to 0.45 and flipped", 64, page, scaled(-0.45, 0.45), ebiten.FilterLinear, false, 1},
			{"and it is squashed below half along one axis only", 64, page, scaled(0.3, 1), ebiten.FilterLinear, false, 0},
			{"and it shrinks to 0.49 but its snapped corners land at exactly half", 10, page, scaled(0.49, 0.49), ebiten.FilterLinear, false, 0},
			{"and a level would halve it to nothing", 4, page, scaled(0.1, 0.1), ebiten.FilterLinear, false, 2},
			{"and its page is too big to have that level", 64, image.Pt(9000, 9000), scaled(0.45, 0.45), ebiten.FilterLinear, false, 0},
		} {
			t.Run(c.name, func(t *testing.T) {
				op := &ebiten.DrawImageOptions{GeoM: c.geo, Filter: c.filter, DisableMipmaps: c.nomip}
				got := imageMipLevel(op, image.Rect(0, 0, c.size, c.size), c.page)
				t.Run(fmt.Sprintf("it should sample level %d", c.want), func(t *testing.T) {
					assert.Equal(t, c.want, got)
				})
			})
		}
	})
	t.Run("when a mesh draw is recorded", func(t *testing.T) {
		tri := func(scale float32, at float32) []ebiten.Vertex {
			return []ebiten.Vertex{
				{DstX: at, DstY: 0, SrcX: 0, SrcY: 0},
				{DstX: at + 64*scale, DstY: 0, SrcX: 64, SrcY: 0},
				{DstX: at, DstY: 64 * scale, SrcX: 0, SrcY: 64},
			}
		}
		for _, c := range []struct {
			name     string
			vertices []ebiten.Vertex
			filter   ebiten.Filter
			want     int
		}{
			{"and it is filtered linear with every triangle at 0.2", append(tri(0.2, 0), tri(0.2, 100)...), ebiten.FilterLinear, 2},
			{"and it is filtered nearest", append(tri(0.2, 0), tri(0.2, 100)...), ebiten.FilterNearest, 0},
			{"and one triangle is at full size and another at 0.2", append(tri(1, 0), tri(0.2, 100)...), ebiten.FilterLinear, 0},
		} {
			t.Run(c.name, func(t *testing.T) {
				op := &ebiten.DrawTrianglesOptions{Filter: c.filter}
				got := meshMipLevel(c.vertices, []uint32{0, 1, 2, 3, 4, 5}, op, page)
				t.Run(fmt.Sprintf("it should sample level %d", c.want), func(t *testing.T) {
					assert.Equal(t, c.want, got)
				})
			})
		}
	})
	t.Run("when a colour-matrix draw is shrunk to a quarter", func(t *testing.T) {
		l := newDrawList()
		src := ebiten.NewImage(64, 64)
		op := &colorm.DrawImageOptions{GeoM: scaled(0.2, 0.2), Filter: ebiten.FilterLinear}
		l.colorm(l.dry, src, nil, colorm.ColorM{}, op)
		t.Run("it should sample level 0", func(t *testing.T) {
			assert.Equal(t, 0, l.mipLevel(0))
		})
	})
}

// Item 236: two draws from one page key together exactly when they
// sample the same level of it.
func TestRecordedDrawsKeyByTheirLevel(t *testing.T) {
	page := ebiten.NewImage(256, 256)
	sprite := page.SubImage(image.Rect(0, 0, 64, 64)).(*ebiten.Image)
	draw := func(l *drawList, scale float64) {
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		op.GeoM.Scale(scale, scale)
		l.ImageOn(l.dry, sprite, page, op)
	}
	for _, c := range []struct {
		name        string
		first, then float64
		together    bool
	}{
		{"and one samples level 1 and the other level 0", 0.45, 1, false},
		{"and both sample level 1", 0.45, 0.4, true},
	} {
		t.Run("when two draws read one page, "+c.name, func(t *testing.T) {
			l := newDrawList()
			draw(l, c.first)
			draw(l, c.then)
			name := "it should key them apart"
			if c.together {
				name = "it should key them together"
			}
			t.Run(name, func(t *testing.T) {
				assert.Equal(t, c.together, l.batchKey(0) == l.batchKey(1))
			})
		})
	}
}

// Item 236: imageMipLevel and meshMipLevel copy a rule private to
// Ebitengine, so an upgrade must re-check the copy.
func TestTheMipmapRuleIsCopiedFromTheEbitengineBuilt(t *testing.T) {
	info, ok := debug.ReadBuildInfo()
	require.True(t, ok, "no build info")
	got := ""
	for _, m := range info.Deps {
		if m.Path == "github.com/hajimehoshi/ebiten/v2" {
			got = m.Version
			if m.Replace != nil {
				got = m.Replace.Version
			}
		}
	}
	t.Run("when the city is built against Ebitengine", func(t *testing.T) {
		t.Run("it should be the version the mipmap rule was copied from", func(t *testing.T) {
			assert.Equal(t, mipRuleEbitengine, got, "Ebitengine moved: re-check internal/mipmap/mipmap.go and canSkipMipmap in image.go against imageMipLevel and meshMipLevel, then update mipRuleEbitengine")
		})
	})
}

// Item 236: Aria ruled sprites drawn without mipmaps (2026-10-24), so no
// draw samples a level of its page, and where a car stands no longer
// decides which level a redrawn rectangle pulls in.
func TestNoDrawSamplesAMipmap(t *testing.T) {
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
			g := c.open(t).game
			require.NotNil(t, g.kits, "no kits were put up")
			kit := map[*ebiten.Image]bool{}
			for _, a := range g.kits.atlases {
				for _, p := range a.pages {
					kit[p] = true
				}
			}
			l := g.rec
			var sprites, mipmapped, levelled int
			for i := range l.cmds {
				cmd := &l.cmds[i]
				if cmd.kind == cmdImage && kit[cmd.page] {
					sprites++
					if !cmd.img.DisableMipmaps {
						mipmapped++
					}
				}
				if l.mipLevel(i) > 0 {
					levelled++
				}
			}
			require.NotEmpty(t, l.cmds, "nothing was recorded")

			t.Run("it should record every kit sprite draw with its mipmaps disabled", func(t *testing.T) {
				assert.Zero(t, mipmapped, "of %d kit sprite draws", sprites)
			})

			t.Run("it should key every draw at mipmap level 0", func(t *testing.T) {
				assert.Zero(t, levelled, "of %d draws", len(l.cmds))
			})
		})
	}
}
