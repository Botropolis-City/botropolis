package render

import (
	"image"
	"math"
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2"
)

// streamPlay is one draw of the record played into one rectangle.
type streamPlay struct {
	cmd, rect int
}

// stream is the order the rectangles something moves in are drawn again
// in, appended to out: every drawable whose box meets one, in the frame's
// order, and each of its draws into every rectangle the draw meets in
// turn. The rectangles do not overlap, so the order across them is free,
// and playing a draw into all of them together lets the same draw merge
// into one, where a rectangle at a time would start each run again
// (item 193).
func stream(out []streamPlay, entries []entry, box func(cmd int) image.Rectangle, rects []image.Rectangle) []streamPlay {
	for _, e := range entries {
		meets := false
		for _, r := range rects {
			if e.box.Overlaps(r) {
				meets = true
				break
			}
		}
		if !meets {
			continue
		}
		for c := e.from; c < e.to; c++ {
			b := box(c)
			for i, r := range rects {
				if b.Overlaps(r) && e.box.Overlaps(r) {
					out = append(out, streamPlay{cmd: c, rect: i})
				}
			}
		}
	}
	return out
}

// batch reorders plays, appended to out, so draws from one source go
// down together: it takes the earliest draw whose source is the one now
// going down and that passes no draw it overlaps in its rectangle, and
// when there is none, the earliest draw that passes none at all. Draws
// that do not overlap touch different pixels, so their order is free;
// draws that do keep theirs, and so the frame is the same (item 193).
func batch(out []streamPlay, plays []streamPlay, box func(cmd int) image.Rectangle, src func(cmd int) int) []streamPlay {
	n := len(plays)
	if n == 0 {
		return out
	}
	// Each draw's box is asked for once: asking again on every
	// comparison was most of the cost (item 161).
	boxes := make([]image.Rectangle, n)
	for j, p := range plays {
		boxes[j] = box(p.cmd)
	}
	return batchBy(out, plays, func(i, j int) bool { return boxes[i].Overlaps(boxes[j]) }, src)
}

// batchBy is batch with the draws that must keep their order told by
// overlap, which is asked of the indices into plays of two draws into
// one rectangle, the earlier first (item 236).
func batchBy(out []streamPlay, plays []streamPlay, overlap func(i, j int) bool, src func(cmd int) int) []streamPlay {
	n := len(plays)
	if n == 0 {
		return out
	}
	// Each draw's source is asked for once (item 161).
	srcs := make([]int, n)
	inRect := map[int][]int{}
	for j, p := range plays {
		srcs[j] = src(p.cmd)
		inRect[p.rect] = append(inRect[p.rect], j)
	}
	// waits[j] counts the earlier draws into j's rectangle that j
	// overlaps and that have not gone down; after lists whom each
	// releases. Only draws into one rectangle are compared.
	waits := make([]int, n)
	after := make([][]int, n)
	for _, js := range inRect {
		for b, j := range js {
			for _, i := range js[:b] {
				if overlap(i, j) {
					waits[j]++
					after[i] = append(after[i], j)
				}
			}
		}
	}
	done := make([]bool, n)
	current, have := 0, false
	for left := n; left > 0; left-- {
		pick := -1
		for j := range plays {
			if done[j] || waits[j] > 0 {
				continue
			}
			if have && srcs[j] == current {
				pick = j
				break
			}
			if pick < 0 {
				pick = j
				if !have {
					break
				}
			}
		}
		done[pick] = true
		current, have = srcs[pick], true
		for _, j := range after[pick] {
			waits[j]--
		}
		out = append(out, plays[pick])
	}
	return out
}

// batchSource is what Ebitengine merges draws on: the page they read
// and the mipmap level of it they sample, each a texture of its own
// (item 236), how they filter and blend, and whether they go through a
// colour matrix. Words are told apart by their face.
type batchSource struct {
	page   uintptr
	level  int
	kind   cmdKind
	filter ebiten.Filter
	blend  ebiten.Blend
	face   unsafe.Pointer
}

// batchKey is a number for the source a recorded draw reads, the same
// for every draw Ebitengine could merge it with.
func (l *drawList) batchKey(i int) int {
	c := &l.cmds[i]
	k := batchSource{kind: c.kind}
	switch c.kind {
	case cmdImage:
		o := &l.imgOps[c.op]
		k.page, k.filter, k.blend = l.pageKey(i), o.Filter, o.Blend
	case cmdColorM:
		o := &l.cmOps[c.op].opts
		k.page, k.filter, k.blend = l.pageKey(i), o.Filter, o.Blend
	case cmdMesh:
		o := &l.triOps[c.op]
		k.page, k.filter, k.blend = l.pageKey(i), o.Filter, o.Blend
	case cmdWords:
		k.face = faceID(l.textOps[c.op].face)
	}
	k.level = l.mipLevel(i)
	if l.batchKeys == nil {
		l.batchKeys = map[batchSource]int{}
	}
	id, ok := l.batchKeys[k]
	if !ok {
		id = len(l.batchKeys)
		l.batchKeys[k] = id
	}
	return id
}

// pageKey is the texture recorded draw i reads: the page its caller cut
// it from, or its own image. It was found by reflecting on Ebitengine's
// private field for the image a sub-image was cut from, on every draw
// of every frame; the caller that cuts a sub-image now says (item 161).
func (l *drawList) pageKey(i int) uintptr {
	c := &l.cmds[i]
	if c.page != nil {
		return uintptr(unsafe.Pointer(c.page))
	}
	return uintptr(unsafe.Pointer(c.src))
}

// mipRuleEbitengine is the Ebitengine whose private rule for which
// mipmap level a draw samples imageMipLevel and meshMipLevel copy:
// canSkipMipmap in image.go and internal/mipmap/mipmap.go (item 236).
const mipRuleEbitengine = "v2.10.2"

// mipLevel is the mipmap level of its page recorded draw i samples.
// A colour-matrix draw goes through DrawRectShader, which samples none.
func (l *drawList) mipLevel(i int) int {
	c := &l.cmds[i]
	switch c.kind {
	case cmdImage:
		return imageMipLevel(&l.imgOps[c.op], c.src.Bounds(), l.pageSize(i))
	case cmdMesh:
		return meshMipLevel(c.vs, c.is, &l.triOps[c.op], l.pageSize(i))
	}
	return 0
}

// pageSize is the size of the texture recorded draw i reads, whose
// mipmaps Ebitengine makes.
func (l *drawList) pageSize(i int) image.Point {
	c := &l.cmds[i]
	if c.page != nil {
		return c.page.Bounds().Size()
	}
	return c.src.Bounds().Size()
}

// imageMipLevel is the level of page an image of bounds src drawn with
// op samples: none unless filtered linear with mipmaps on and shrunk,
// and then the lowest any edge of its two triangles asks for, from its
// corners snapped as Ebitengine snaps them.
func imageMipLevel(op *ebiten.DrawImageOptions, src image.Rectangle, page image.Point) int {
	if op.Filter != ebiten.FilterLinear || op.DisableMipmaps {
		return 0
	}
	g := op.GeoM
	// GeoM keeps a and d less one, and Ebitengine adds the one back
	// after narrowing to float32.
	a, b := float32(g.Element(0, 0)-1)+1, float32(g.Element(0, 1))
	c, d := float32(g.Element(1, 0)), float32(g.Element(1, 1)-1)+1
	tx, ty := float32(g.Element(0, 2)), float32(g.Element(1, 2))
	if math.Abs(float64(a*d-b*c)) >= 0.999 {
		return 0
	}
	sx0, sy0, sx1, sy1 := float32(src.Min.X), float32(src.Min.Y), float32(src.Max.X), float32(src.Max.Y)
	x, y := sx1-sx0, sy1-sy0
	ax, by, cx, dy := a*x, b*y, c*x, d*y
	vs := [4][4]float32{
		{snapThird(tx), snapThird(ty), sx0, sy0},
		{snapThird(ax + tx), snapThird(cx + ty), sx1, sy0},
		{snapThird(by + tx), snapThird(dy + ty), sx0, sy1},
		{snapThird(ax + by + tx), snapThird(cx + dy + ty), sx1, sy1},
	}
	level := math.MaxInt32
	for _, t := range [2][3]int{{0, 1, 2}, {1, 2, 3}} {
		level = min(level, triangleMipLevel(vs[t[0]], vs[t[1]], vs[t[2]]))
	}
	return pageHasLevel(page, level)
}

// meshMipLevel is the level of page triangles drawn with op sample:
// none unless filtered linear with mipmaps on, and then the lowest any
// edge of any triangle asks for.
func meshMipLevel(vertices []ebiten.Vertex, indices []uint32, op *ebiten.DrawTrianglesOptions, page image.Point) int {
	if op.Filter != ebiten.FilterLinear || op.DisableMipmaps || len(indices) < 3 {
		return 0
	}
	at := func(i uint32) [4]float32 {
		v := vertices[i]
		return [4]float32{v.DstX, v.DstY, v.SrcX, v.SrcY}
	}
	level := math.MaxInt32
	for i := 0; i+2 < len(indices); i += 3 {
		level = min(level, triangleMipLevel(at(indices[i]), at(indices[i+1]), at(indices[i+2])))
	}
	return pageHasLevel(page, level)
}

func triangleMipLevel(p0, p1, p2 [4]float32) int {
	return min(edgeMipLevel(p0, p1), edgeMipLevel(p1, p2), edgeMipLevel(p2, p0))
}

// edgeMipLevel is Ebitengine's mipmapLevelFromDistance for the edge
// from p to q, each {dstX, dstY, srcX, srcY}: one level each time the
// edge is drawn below half its length, short of halving it to nothing.
func edgeMipLevel(p, q [4]float32) int {
	const maxLevel = 6
	d := (q[0]-p[0])*(q[0]-p[0]) + (q[1]-p[1])*(q[1]-p[1])
	s := (q[2]-p[2])*(q[2]-p[2]) + (q[3]-p[3])*(q[3]-p[3])
	if s == 0 {
		return 0
	}
	scale := d / s
	if math.IsInf(float64(scale), 0) || scale == 0 {
		return 0
	}
	level := 0
	for scale < 0.25 {
		level++
		scale *= 4
	}
	if level > 0 {
		w, h := int(q[2]-p[2]), int(q[3]-p[3])
		for level >= 0 {
			if (w > 0 && halved(w, level) == 0) || (h > 0 && halved(h, level) == 0) {
				level--
				continue
			}
			break
		}
		if level < 0 {
			return 0
		}
	}
	return min(level, maxLevel)
}

// pageHasLevel is level, or 0 where Ebitengine makes no image for that
// level of page, too small or too big, and draws from the page itself.
func pageHasLevel(page image.Point, level int) int {
	if level <= 0 {
		return 0
	}
	if halved(page.X, level) == 0 || halved(page.Y, level) == 0 || halved(page.X, 1) > 4096 || halved(page.Y, 1) > 4096 {
		return 0
	}
	return level
}

// halved is x halved level times, as Ebitengine sizes a mipmap level.
func halved(x, level int) int {
	for range level {
		x /= 2
		if x == 0 {
			return 0
		}
	}
	return x
}

// snapThird is Ebitengine's adjustDestinationPixel: a quad's corners
// are moved off the centres of pixels, to about a third of one.
func snapThird(x float32) float32 {
	ix := float32(int(x))
	if x < 0 && x != ix {
		ix--
	}
	switch frac := x - ix; {
	case frac < 3.0/16.0:
		return ix
	case frac < 8.0/16.0:
		return ix + 5.0/16.0
	case frac < 13.0/16.0:
		return ix + 11.0/16.0
	default:
		return ix + 1
	}
}
