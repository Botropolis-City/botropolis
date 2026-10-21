package render

import (
	"encoding/binary"
	"hash/maphash"
	"image"
	"math"
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2"
)

// The standing city (item 193). The dry pass records every drawable of
// the frame in depth order; the still ones are composed over the ground
// into one image while what they would paint holds, and each frame is
// that image once, then every rectangle something moves in drawn again
// from the ground up, clipped: inside it, the same draws in the same
// order as drawing the whole frame, so a mover behind a building stays
// behind it. The night's lights are kept and replayed the same way, and
// the group lines, which go over everything, are a layer of their own.

// stillFrame is the standing city composed, and what it was composed
// for.
type stillFrame struct {
	img, lights, overlay *ebiten.Image
	key                  stillKey
	ok                   bool
	// held is the key the last frame asked for, and direct whether this
	// frame's was new, so the city was drawn straight instead.
	held   stillKey
	direct bool
	// set is the still drawables the layer holds, changed what differs
	// this frame and redo the rectangles composed again for it.
	set           *stillSet
	changed, redo []image.Rectangle
	overlayHash   uint64
	overlayHeld   uint64
	overlayOK     bool

	entries []entry
	boxes   []image.Rectangle
	rects   []image.Rectangle
	ops     []lightOp
	opsHash maphash.Hash

	stats stillStats
}

// stillKey is what the standing city is composed over and through that
// what its drawables would paint does not hold: the ground, by what it
// painted, and whether a view drains the colour from it.
type stillKey struct {
	under   uint64
	viewing bool
}

// stillStats is what the standing city cost a frame: how often it was
// composed whole, in how many rectangles again, whether the group lines
// were, and how many rectangles of movers were drawn again.
type stillStats struct {
	composes, redos, overlays, rects, direct int
}

// note runs a drawable: onto the frame when nothing is recorded, or in
// the dry pass, keeping what it would draw, the light ops it noted, and
// whether it moves.
func (g *Game) note(draw func(), moves bool) {
	if g.noStill {
		draw()
		return
	}
	l := g.rec
	first := len(g.lightOps)
	// Each drawable's mesh cuts are its own, so they can be laid again
	// with it alone.
	g.meshCut = -1
	l.begin()
	l.moving = false
	draw()
	e := entry{inked: l.end(), moves: moves || l.moving, lights: [2]int{first, len(g.lightOps)}}
	if first < len(g.lightOps) {
		e.box, e.hash = g.withLights(e, g.lightOps[first:])
	}
	g.still.entries = append(g.still.entries, e)
}

// withLights widens a drawable's box by the lights it noted and folds
// them into its hash: a window lit or put out changes the lights even
// when nothing it draws does.
func (g *Game) withLights(e entry, ops []lightOp) (image.Rectangle, uint64) {
	h := &g.still.opsHash
	h.SetSeed(inkSeed)
	h.Reset()
	var b [8]byte
	put := func(v uint64) {
		binary.LittleEndian.PutUint64(b[:], v)
		h.Write(b[:])
	}
	put(e.hash)
	box := e.box
	for _, op := range ops {
		box = box.Union(padded(op.box))
		if op.punch || op.cut != cutNone {
			put(uint64(op.cut))
			put(uint64(uintptr(unsafe.Pointer(op.src))))
			for i := range 2 {
				for j := range 3 {
					put(math.Float64bits(op.geo.Element(i, j)))
				}
			}
			put(uint64(math.Float32bits(op.alpha)))
			h.WriteString(op.words)
			for _, v := range []int{op.box.Min.X, op.box.Min.Y, op.box.Max.X, op.box.Max.Y} {
				put(uint64(int64(v)))
			}
			continue
		}
		for _, r := range g.litRects[op.from:op.to] {
			box = box.Union(padded(r))
		}
		put(uint64(uintptr(unsafe.Pointer(op.src))))
		if op.warm {
			put(1)
		}
		if len(op.vertices) > 0 {
			h.Write(unsafe.Slice((*byte)(unsafe.Pointer(&op.vertices[0])), len(op.vertices)*int(unsafe.Sizeof(op.vertices[0]))))
		}
		if len(op.indices) > 0 {
			h.Write(unsafe.Slice((*byte)(unsafe.Pointer(&op.indices[0])), len(op.indices)*2))
		}
	}
	return box, h.Sum64()
}

// padded is a light's box widened as a draw's is, for the filtering
// that spreads it a pixel; an empty box stays empty, or it would widen
// whatever it joins to the frame's corner.
func padded(r image.Rectangle) image.Rectangle {
	if r.Empty() {
		return r
	}
	return r.Inset(-inkPad)
}

// paintStill puts the standing city on the frame, composing it first:
// whole when its key or size has changed, and otherwise only inside the
// rectangles where a still drawable would paint differently. Then it
// draws again every rectangle something moves in.
func (g *Game) paintStill(real, under *ebiten.Image, key stillKey) {
	s := &g.still
	size := real.Bounds().Size()
	if s.set == nil {
		s.set = newStillSet(nil)
	}
	whole := !s.ok || s.key != key || s.img == nil || s.img.Bounds().Size() != size || g.lightsOn && s.lights == nil
	s.direct = whole && key != s.held
	s.held = key
	if s.direct {
		// A key new this frame changes every frame while the camera
		// moves, and composing the city each frame to blit it once
		// costs more than drawing it, so the record is played straight
		// onto the frame until a key holds for one, as a building's
		// look is baked only once its key has held (item 192).
		s.ok = false
		g.blitBase(real, under)
		f := g.newFadePlay(real)
		for _, e := range s.entries {
			g.rec.play(real, e.inked, f)
		}
		f.done()
		s.rects = s.rects[:0]
		s.stats.direct = 1
		return
	}
	if whole {
		s.img = sizedImage(s.img, size)
		if g.lightsOn {
			s.lights = sizedImage(s.lights, size)
		}
		g.composeStill(s.img.Bounds(), under)
		s.stats.composes++
	} else {
		s.changed = s.set.changes(s.changed[:0], s.entries)
		s.redo = dirtyRects(s.redo[:0], s.changed, s.img.Bounds())
		for _, r := range s.redo {
			g.composeStill(r, under)
		}
		s.stats.redos = len(s.redo)
	}
	s.set.reset(s.entries)
	s.key, s.ok = key, true
	real.DrawImage(s.img, nil)
	s.boxes = s.boxes[:0]
	for _, e := range s.entries {
		if e.moves {
			s.boxes = append(s.boxes, e.box)
		}
	}
	s.rects = dirtyRects(s.rects[:0], s.boxes, real.Bounds())
	s.stats.rects = len(s.rects)
	for _, r := range s.rects {
		sub := real.SubImage(r).(*ebiten.Image)
		g.blitBase(sub, under)
		f := g.newFadePlay(sub)
		for _, e := range s.entries {
			if e.box.Overlaps(r) {
				g.rec.play(sub, e.inked, f)
			}
		}
		f.done()
	}
}

// composeStill composes the standing city inside r: the ground, and
// every still drawable whose box meets it, in the frame's order, and by
// night their lights.
func (g *Game) composeStill(r image.Rectangle, under *ebiten.Image) {
	s := &g.still
	sub := s.img.SubImage(r).(*ebiten.Image)
	g.blitBase(sub, under)
	f := g.newFadePlay(sub)
	for _, e := range s.entries {
		if !e.moves && e.box.Overlaps(r) {
			g.rec.play(sub, e.inked, f)
		}
	}
	f.done()
	if g.lightsOn {
		lights := s.lights.SubImage(r).(*ebiten.Image)
		lights.Clear()
		g.layLights(lights, g.opsOf(&r, true))
	}
}

// paintLights lays the night's lights over the frame: the standing
// city's, kept, and inside each rectangle something moves in, laid
// again from the ops of what is drawn there.
func (g *Game) paintLights(real *ebiten.Image) {
	if !g.lightsOn {
		return
	}
	g.lightsOn = false
	g.paint.Cut = nil
	if len(g.lightOps) == 0 {
		return
	}
	s := &g.still
	g.lights = sizedImage(g.lights, real.Bounds().Size())
	if s.direct {
		g.lights.Clear()
		g.layLights(g.lights, g.lightOps)
		real.DrawImage(g.lights, nil)
		return
	}
	g.lights.DrawImage(s.lights, &ebiten.DrawImageOptions{Blend: ebiten.BlendCopy})
	for _, r := range s.rects {
		sub := g.lights.SubImage(r).(*ebiten.Image)
		sub.Clear()
		g.layLights(sub, g.opsOf(&r, false))
	}
	real.DrawImage(g.lights, nil)
}

// opsOf is the light ops, in the order noted, of the still drawables, or
// of every drawable whose box meets r.
func (g *Game) opsOf(r *image.Rectangle, stillOnly bool) []lightOp {
	s := &g.still
	s.ops = s.ops[:0]
	for _, e := range s.entries {
		if stillOnly && e.moves || r != nil && !e.box.Overlaps(*r) {
			continue
		}
		s.ops = append(s.ops, g.lightOps[e.lights[0]:e.lights[1]]...)
	}
	return s.ops
}

// paintOverlay puts the group lines over the frame from a layer of
// their own, composed again when what they would paint changes.
func (g *Game) paintOverlay(real *ebiten.Image, in inked) {
	if in.to == in.from {
		return
	}
	s := &g.still
	size := real.Bounds().Size()
	whole := !s.overlayOK || s.overlayHash != in.hash || s.overlay == nil || s.overlay.Bounds().Size() != size
	held := s.overlayHeld == in.hash
	s.overlayHeld = in.hash
	if whole && !held {
		// New this frame, as every frame's is while the camera moves:
		// drawn straight until it holds for one.
		s.overlayOK = false
		f := g.newFadePlay(real)
		g.rec.play(real, in, f)
		f.done()
		return
	}
	if whole {
		s.overlay = sizedImage(s.overlay, size)
		s.overlay.Clear()
		f := g.newFadePlay(s.overlay)
		g.rec.play(s.overlay, in, f)
		f.done()
		s.overlayHash, s.overlayOK = in.hash, true
		s.stats.overlays++
	}
	real.DrawImage(s.overlay, nil)
}

// sizedImage is img, or a new image when it is not size. A layer the
// frame's size has a texture of its own: on Ebitengine's shared atlas,
// beside the sprites the night's lights are cut with, a cut laid onto
// the still lights left a lit pixel dark that the same cuts laid onto a
// new image kept (item 193).
func sizedImage(img *ebiten.Image, size image.Point) *ebiten.Image {
	if img != nil && img.Bounds().Size() == size {
		return img
	}
	if img != nil {
		img.Deallocate()
	}
	return ebiten.NewImageWithOptions(image.Rectangle{Max: size}, &ebiten.NewImageOptions{Unmanaged: true})
}
