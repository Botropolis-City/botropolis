package render

import (
	"image"
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/colorm"
)

// Item 328: Ebitengine takes every draw call's uniforms from a buffer
// its command queue keeps at the most any frame has asked of it, even a
// call it then merges with the one before. A frame that composes the
// whole standing city made one call a recorded draw, about 17,000 in
// the city of 600 districts, and left each queue holding several MiB
// more, a queue at a time. So the standing city is played a run at a
// time: draws one after another from one page, through one blend,
// filter, mipmap level and colour matrix, go down as one call of the
// triangles each would have drawn, which is the draw Ebitengine would
// have merged them into.

// stillRun is the run being gathered: its triangles and how they go
// down, through a colour matrix when through is set.
type stillRun struct {
	open    bool
	key     int
	src     *ebiten.Image
	op      ebiten.DrawTrianglesOptions
	through bool
	cm      colorm.ColorM
	vs      []ebiten.Vertex
	is      []uint32
	is16    []uint16
}

// geoMElements mirrors ebiten.GeoM, whose elements it keeps less one on
// the diagonal, so a quad is made from the very float32s DrawImage
// makes it from.
type geoMElements struct{ a1, b, c, d1, tx, ty float64 }

// beginRuns plays the record onto dst a run at a time until endRuns.
func (l *drawList) beginRuns(dst *ebiten.Image) {
	l.runDst = dst
}

// endRuns puts down the last run and plays a draw at a time again.
func (l *drawList) endRuns() {
	l.flushRun()
	l.runDst = nil
}

// playRun adds recorded draw i to the run, putting the run down first
// when i cannot join it; a draw that cannot go down as triangles goes
// down alone.
func (l *drawList) playRun(dst *ebiten.Image, i int) {
	c := &l.cmds[i]
	op := ebiten.DrawTrianglesOptions{ColorScaleMode: ebiten.ColorScaleModePremultipliedAlpha}
	var cm *colorm.ColorM
	switch c.kind {
	case cmdImage:
		o := &l.imgOps[c.op]
		op.Filter, op.Blend = o.Filter, o.Blend
	case cmdColorM:
		o := &l.cmOps[c.op]
		op.Filter, op.Blend, cm = o.opts.Filter, o.opts.Blend, &o.cm
	case cmdMesh:
		o := &l.triOps[c.op]
		op.Filter, op.Blend, op.Address = o.Filter, o.Blend, o.Address
	default:
		l.playAlone(dst, c)
		return
	}
	if l.mipLevel(i) == 0 {
		op.DisableMipmaps = true
	}
	src, key := l.pageOf(c), l.batchKey(i)
	r := &l.run
	if r.open && (r.key != key || r.src != src || !sameRunOp(&r.op, &op) || r.through != (cm != nil) ||
		cm != nil && !sameColorM(&r.cm, cm) || len(r.vs)+len(c.vs)+4 > runVertexLimit) {
		l.flushRun()
	}
	if !r.open {
		r.open, r.key, r.src, r.op, r.through = true, key, src, op, cm != nil
		if cm != nil {
			r.cm = *cm
		}
		r.vs, r.is = r.vs[:0], r.is[:0]
	}
	switch c.kind {
	case cmdImage:
		o := &l.imgOps[c.op]
		l.appendQuad(&o.GeoM, c.src.Bounds(), &o.ColorScale)
		return
	case cmdColorM:
		o := &l.cmOps[c.op]
		l.appendQuad(&o.opts.GeoM, c.src.Bounds(), &o.opts.ColorScale)
		return
	}
	base := uint32(len(r.vs))
	straight := l.triOps[c.op].ColorScaleMode == ebiten.ColorScaleModeStraightAlpha
	for _, v := range c.vs {
		if straight {
			v.ColorR, v.ColorG, v.ColorB = v.ColorR*v.ColorA, v.ColorG*v.ColorA, v.ColorB*v.ColorA
		}
		r.vs = append(r.vs, v)
	}
	for _, x := range c.is {
		r.is = append(r.is, base+x)
	}
}

// runVertexLimit holds a run to what a call takes, and to what a
// colour-matrix call's 16-bit indices can reach.
const runVertexLimit = 1 << 16

// playAlone puts down the run, then draw c by itself.
func (l *drawList) playAlone(dst *ebiten.Image, c *drawCmd) {
	l.flushRun()
	l.playCmd(dst, c)
}

// appendQuad adds the two triangles DrawImage, or DrawRectShader for a
// colour-matrix draw, would draw of src through geo at scale, and none
// for a draw either would skip.
func (l *drawList) appendQuad(geo *ebiten.GeoM, src image.Rectangle, scale *ebiten.ColorScale) {
	g := (*geoMElements)(unsafe.Pointer(geo))
	a, b, c, d := float32(g.a1)+1, float32(g.b), float32(g.c), float32(g.d1)+1
	tx, ty := float32(g.tx), float32(g.ty)
	if a*d-b*c == 0 {
		return
	}
	sx0, sy0, sx1, sy1 := float32(src.Min.X), float32(src.Min.Y), float32(src.Max.X), float32(src.Max.Y)
	x, y := sx1-sx0, sy1-sy0
	ax, by, cx, dy := a*x, b*y, c*x, d*y
	cr, cg, cb, ca := scale.R(), scale.G(), scale.B(), scale.A()
	r := &l.run
	base := uint32(len(r.vs))
	r.vs = append(r.vs,
		ebiten.Vertex{DstX: snapThird(tx), DstY: snapThird(ty), SrcX: sx0, SrcY: sy0, ColorR: cr, ColorG: cg, ColorB: cb, ColorA: ca},
		ebiten.Vertex{DstX: snapThird(ax + tx), DstY: snapThird(cx + ty), SrcX: sx1, SrcY: sy0, ColorR: cr, ColorG: cg, ColorB: cb, ColorA: ca},
		ebiten.Vertex{DstX: snapThird(by + tx), DstY: snapThird(dy + ty), SrcX: sx0, SrcY: sy1, ColorR: cr, ColorG: cg, ColorB: cb, ColorA: ca},
		ebiten.Vertex{DstX: snapThird(ax + by + tx), DstY: snapThird(cx + dy + ty), SrcX: sx1, SrcY: sy1, ColorR: cr, ColorG: cg, ColorB: cb, ColorA: ca},
	)
	r.is = append(r.is, base, base+1, base+2, base+1, base+2, base+3)
}

// pageOf is the texture recorded draw c reads: its page, or its own
// image.
func (l *drawList) pageOf(c *drawCmd) *ebiten.Image {
	if c.page != nil {
		return c.page
	}
	return c.src
}

// flushRun puts down the run gathered, as one call.
func (l *drawList) flushRun() {
	r := &l.run
	if !r.open {
		return
	}
	r.open = false
	if len(r.is) == 0 {
		return
	}
	l.calls++
	if !r.through {
		l.runDst.DrawTriangles32(r.vs, r.is, r.src, &r.op)
		return
	}
	r.is16 = r.is16[:0]
	for _, x := range r.is {
		r.is16 = append(r.is16, uint16(x))
	}
	colorm.DrawTriangles(l.runDst, r.vs, r.is16, r.src, r.cm, &colorm.DrawTrianglesOptions{
		ColorScaleMode: ebiten.ColorScaleModePremultipliedAlpha,
		Blend:          r.op.Blend,
		Filter:         r.op.Filter,
	})
}

// sameColorM is whether two colour matrices are the same matrix.
func sameColorM(a, b *colorm.ColorM) bool {
	for i := range 4 {
		for j := range 5 {
			if a.Element(i, j) != b.Element(i, j) {
				return false
			}
		}
	}
	return true
}

// sameRunOp is whether two runs' options, which are all a run sets, go
// down alike.
func sameRunOp(a, b *ebiten.DrawTrianglesOptions) bool {
	return a.Filter == b.Filter && a.Blend == b.Blend && a.Address == b.Address &&
		a.ColorScaleMode == b.ColorScaleMode && a.DisableMipmaps == b.DisableMipmaps
}
