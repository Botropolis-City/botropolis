package render

import (
	"encoding/binary"
	"hash/maphash"
	"image"
	"math"
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/colorm"

	"botropolis.city/botropolis/pkg/city"
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
	// origin is the camera's origin the layer was composed at, and
	// heldOrigin the last frame's (item 193).
	origin, heldOrigin city.Point
	// scratch is a frame-sized image a layer is shifted through; it
	// then takes the layer's old pixels' place. unshifted counts the
	// frames since the last shift through it.
	scratch   *ebiten.Image
	unshifted int
	// overlayOrigin is the origin the group lines were composed at.
	overlayOrigin city.Point
	// set is the still drawables the layer holds, changed what differs
	// this frame and redo the rectangles composed again for it.
	set               *stillSet
	changed, redo     []image.Rectangle
	overlayHash       uint64
	overlayHeld       uint64
	overlayHeldOrigin city.Point
	overlayOK         bool

	entries   []entry
	boxes     []image.Rectangle
	subs      []*ebiten.Image
	plays     []streamPlay
	batched   []streamPlay
	lit       []entry
	pend      [][]int
	panes     []uint16
	due       []leftCut
	left      []leftCut
	laid      []streamPlay
	unbatched []streamPlay
	lightKeys map[lightSource]int
	rects     []image.Rectangle
	ops       []lightOp
	opsHash   maphash.Hash

	stats stillStats
}

// stillKey is what the standing city is composed over and through that
// what its drawables would paint does not hold: the ground, by how often
// it was painted whole, and whether a view drains the colour from it.
type stillKey struct {
	under   uint64
	viewing bool
}

// stillStats is what the standing city cost a frame: how often it was
// composed whole, in how many rectangles again, whether the group lines
// were, and how many rectangles of movers were drawn again.
type stillStats struct {
	composes, redos, overlays, rects, direct, shifts int
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
	l.moving, l.active = false, false
	draw()
	e := entry{inked: l.end(), moves: moves || l.moving, active: moves || l.active, lights: [2]int{first, len(g.lightOps)}}
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
	// Positions relative to the camera's origin, as a drawable's draws
	// are hashed, so a whole-pixel pan changes nothing here either.
	o := g.rec.origin
	box := e.box
	for _, op := range ops {
		box = box.Union(padded(op.box))
		if op.punch || op.cut != cutNone {
			put(uint64(op.cut))
			put(uint64(uintptr(unsafe.Pointer(op.src))))
			for i := range 2 {
				for j := range 2 {
					put(uint64(int64(math.Round(op.geo.Element(i, j) * (1 << 24)))))
				}
			}
			put(relQ(op.geo.Element(0, 2), o.X))
			put(relQ(op.geo.Element(1, 2), o.Y))
			put(uint64(math.Float32bits(op.alpha)))
			h.WriteString(op.words)
			put(relQ(float64(op.box.Min.X), o.X))
			put(relQ(float64(op.box.Min.Y), o.Y))
			put(relQ(float64(op.box.Max.X), o.X))
			put(relQ(float64(op.box.Max.Y), o.Y))
			continue
		}
		for _, r := range g.litRects[op.from:op.to] {
			box = box.Union(padded(r))
		}
		put(uint64(uintptr(unsafe.Pointer(op.src))))
		if op.warm {
			put(1)
		}
		for _, v := range op.vertices {
			put(relQ(float64(v.DstX), o.X))
			put(relQ(float64(v.DstY), o.Y))
			put(uint64(math.Float32bits(v.SrcX))<<32 | uint64(math.Float32bits(v.SrcY)))
			put(uint64(math.Float32bits(v.ColorR))<<32 | uint64(math.Float32bits(v.ColorA)))
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
	g.restScratch()
	s := &g.still
	size := real.Bounds().Size()
	if s.set == nil {
		s.set = newStillSet(nil)
	}
	origin := g.rec.origin
	whole := !s.ok || s.key != key || s.img == nil || s.img.Bounds().Size() != size || g.lightsOn && s.lights == nil
	var shift image.Point
	if !whole && origin != s.origin {
		k, ok := wholeShift(s.origin, origin, s.img.Bounds())
		whole, shift = !ok, k
	}
	s.direct = whole && (key != s.held || origin != s.heldOrigin)
	s.held, s.heldOrigin = key, origin
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
		s.changed = s.changed[:0]
		if shift != (image.Point{}) {
			g.shiftLayer(&s.img, shift)
			if s.lights != nil {
				g.shiftLayer(&s.lights, shift)
			}
			s.set.shift(shift)
			s.changed = append(s.changed, exposed(s.img.Bounds(), shift)...)
			s.stats.shifts++
		}
		// Where the ground under it was painted again, so is it.
		s.changed = append(s.changed, g.static.redo...)
		s.changed = s.set.changes(s.changed, s.entries)
		s.redo = dirtyRects(s.redo[:0], s.changed, s.img.Bounds())
		for _, r := range s.redo {
			g.composeStill(r, under)
		}
		s.stats.redos = len(s.redo)
	}
	s.set.reset(s.entries)
	s.key, s.origin, s.ok = key, origin, true
	real.DrawImage(s.img, nil)
	s.boxes = s.boxes[:0]
	for _, e := range s.entries {
		if e.moves {
			s.boxes = append(s.boxes, e.box)
		}
	}
	s.rects = dirtyRects(s.rects[:0], s.boxes, real.Bounds())
	s.stats.rects = len(s.rects)
	s.subs = s.subs[:0]
	for _, r := range s.rects {
		sub := real.SubImage(r).(*ebiten.Image)
		g.blitBase(sub, under)
		s.subs = append(s.subs, sub)
	}
	if g.lit.RecedesGround() {
		// The fade's layer goes down a rectangle at a time.
		for i, r := range s.rects {
			f := g.newFadePlay(s.subs[i])
			for _, e := range s.entries {
				if e.box.Overlaps(r) {
					g.rec.play(s.subs[i], e.inked, f)
				}
			}
			f.done()
		}
		return
	}
	s.plays = stream(s.plays[:0], s.entries, g.rec.cmdBox, s.rects)
	s.batched = batch(s.batched[:0], s.plays, g.rec.cmdBox, g.rec.batchKey)
	for _, p := range s.batched {
		g.rec.playCmd(s.subs[p.rect], &g.rec.cmds[p.cmd])
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
	s.subs = s.subs[:0]
	for _, r := range s.rects {
		sub := g.lights.SubImage(r).(*ebiten.Image)
		sub.Clear()
		s.subs = append(s.subs, sub)
	}
	// The ops of what is drawn in the rectangles, played as one stream,
	// as its draws were.
	s.lit = s.lit[:0]
	for _, e := range s.entries {
		s.lit = append(s.lit, entry{inked: inked{from: e.lights[0], to: e.lights[1], box: e.box}})
	}
	s.plays = stream(s.plays[:0], s.lit, g.lightBox, s.rects)
	g.layLightsStream(s.subs, s.plays)
	real.DrawImage(g.lights, nil)
}

// layLightsStream lays light ops into the rectangles they were played
// into, in the order lightPlays gives.
func (g *Game) layLightsStream(subs []*ebiten.Image, plays []streamPlay) {
	s := &g.still
	s.laid = g.lightPlays(s.laid[:0], plays)
	for _, p := range s.laid {
		op, dst := g.lightOps[p.cmd], subs[p.rect]
		if op.punch || op.cut != cutNone {
			g.layCut(dst, op)
			continue
		}
		indices := g.panesIn(op, s.rects[p.rect])
		if op.warm {
			colorm.DrawTriangles(dst, op.vertices, indices, op.src, warmGlass, warmOptions)
			continue
		}
		dst.DrawTriangles(op.vertices, indices, op.src, windowOptions)
	}
}

// lightPlays is the order light ops played into rectangles are laid
// in, appended to out: the order lightOrder gives, batched by source as
// the mover pass is, so a window passes only draws its panes do not
// overlap in its rectangle, and a cut only those and other cuts, since
// cuts only take light away; and the lights come out the same (item
// 236).
func (g *Game) lightPlays(out, plays []streamPlay) []streamPlay {
	s := &g.still
	s.unbatched = g.lightOrder(s.unbatched[:0], plays)
	clear(s.lightKeys)
	return batchBy(out, s.unbatched, func(i, j int) bool {
		return g.lightsOverlap(s.unbatched[i].cmd, s.unbatched[j].cmd)
	}, g.lightKey)
}

// lightsOverlap is whether light ops a and b must be laid in the order
// noted: neither is a cut, or one is, and its box meets the other's,
// a window's being each of its panes with the pixels its light spreads.
func (g *Game) lightsOverlap(a, b int) bool {
	x, y := &g.lightOps[a], &g.lightOps[b]
	xCut, yCut := x.punch || x.cut != cutNone, y.punch || y.cut != cutNone
	switch {
	case xCut && yCut:
		return false
	case xCut:
		return g.panesMeet(y, padded(x.box))
	case yCut:
		return g.panesMeet(x, padded(y.box))
	}
	if !g.lightBox(a).Overlaps(g.lightBox(b)) {
		return false
	}
	for _, p := range g.litRects[x.from:x.to] {
		if g.panesMeet(y, padded(p)) {
			return true
		}
	}
	return false
}

// panesMeet is whether any of window op's panes, with the pixels its
// light spreads, meets box.
func (g *Game) panesMeet(op *lightOp, box image.Rectangle) bool {
	for _, p := range g.litRects[op.from:op.to] {
		if padded(p).Overlaps(box) {
			return true
		}
	}
	return false
}

// lightSource is what Ebitengine merges light draws on: what is drawn,
// which says the shader and the blend, and the texture or face read.
type lightSource struct {
	kind int
	img  *ebiten.Image
	face unsafe.Pointer
}

// lightKey is a number for the source light op i is laid from, the same
// for every light op Ebitengine could lay with it in one draw.
func (g *Game) lightKey(i int) int {
	op := &g.lightOps[i]
	k := lightSource{kind: 1 + int(op.cut), img: op.src}
	switch {
	case op.punch:
		k = lightSource{kind: -1, img: op.page}
	case op.cut == cutNone && op.warm:
		k.kind = -2
	case op.cut == cutWords:
		k = lightSource{kind: k.kind, face: faceID(op.face)}
	case op.cut == cutMesh || op.cut == cutRing:
		k.img = nil
	}
	s := &g.still
	if s.lightKeys == nil {
		s.lightKeys = map[lightSource]int{}
	}
	id, ok := s.lightKeys[k]
	if !ok {
		id = len(s.lightKeys)
		s.lightKeys[k] = id
	}
	return id
}

// lightOrder is the order light ops played into rectangles are laid
// in before batching, appended to out: each rectangle's in the order
// noted, a cut waiting, in its own rectangle, for the next window there
// it meets, as layLights has it, and what is left over at the end in
// orderCuts' order across every rectangle, since cuts only take light
// away and their order among themselves does not matter (items 193 and
// 199).
func (g *Game) lightOrder(out, plays []streamPlay) []streamPlay {
	s := &g.still
	rects := 0
	for _, p := range plays {
		rects = max(rects, p.rect+1)
	}
	for len(s.pend) < rects {
		s.pend = append(s.pend, nil)
	}
	for i := range rects {
		s.pend[i] = s.pend[i][:0]
	}
	cut := func(c *leftCut) *lightOp { return &g.lightOps[c.cmd] }
	for _, p := range plays {
		op, r := &g.lightOps[p.cmd], p.rect
		if op.punch || op.cut != cutNone {
			s.pend[r] = append(s.pend[r], p.cmd)
			continue
		}
		due, kept := s.due[:0], s.pend[r][:0]
		for _, c := range s.pend[r] {
			if meets(g.lightOps[c].box, g.litRects[op.from:op.to]) {
				due = append(due, leftCut{cmd: c, rect: r})
			} else {
				kept = append(kept, c)
			}
		}
		orderCutsBy(due, cut)
		for _, c := range due {
			out = append(out, streamPlay{cmd: c.cmd, rect: r})
		}
		s.due, s.pend[r] = due[:0], kept
		out = append(out, p)
	}
	s.left = s.left[:0]
	for r := range rects {
		for _, c := range s.pend[r] {
			s.left = append(s.left, leftCut{cmd: c, rect: r})
		}
	}
	orderCutsBy(s.left, cut)
	for _, c := range s.left {
		out = append(out, streamPlay{cmd: c.cmd, rect: c.rect})
	}
	return out
}

// lightBox is the box the ith light op covers: a cut's own, or a
// window's panes with the pixel their light spreads past them.
func (g *Game) lightBox(i int) image.Rectangle {
	op := &g.lightOps[i]
	if op.punch || op.cut != cutNone {
		return padded(op.box)
	}
	var box image.Rectangle
	for _, r := range g.litRects[op.from:op.to] {
		box = box.Union(padded(r))
	}
	return box
}

// panesIn is the indices of a window op's panes whose light reaches r:
// the rest would light nothing there.
func (g *Game) panesIn(op lightOp, r image.Rectangle) []uint16 {
	panes := op.to - op.from
	if panes == 0 || len(op.indices)%panes != 0 {
		return op.indices
	}
	step := len(op.indices) / panes
	s := &g.still
	s.panes = s.panes[:0]
	for i, p := range g.litRects[op.from:op.to] {
		if padded(p).Overlaps(r) {
			s.panes = append(s.panes, op.indices[i*step:(i+1)*step]...)
		}
	}
	return s.panes
}

// leftCut is a cut still to be laid, and the rectangle it is laid in.
type leftCut struct {
	cmd, rect int
}

// cutOrder is the order layCuts lays cuts in, by kind.
func cutOrder(op lightOp) int {
	if op.punch {
		return 0
	}
	return int(op.cut)
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
	origin := g.rec.origin
	whole := !s.overlayOK || s.overlayHash != in.hash || s.overlay == nil || s.overlay.Bounds().Size() != size
	var shift image.Point
	if !whole && origin != s.overlayOrigin {
		k, ok := wholeShift(s.overlayOrigin, origin, s.overlay.Bounds())
		whole, shift = !ok, k
	}
	held := s.overlayHeld == in.hash && origin == s.overlayHeldOrigin
	s.overlayHeld, s.overlayHeldOrigin = in.hash, origin
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
	} else if shift != (image.Point{}) {
		g.shiftLayer(&s.overlay, shift)
		for _, r := range exposed(s.overlay.Bounds(), shift) {
			sub := s.overlay.SubImage(r).(*ebiten.Image)
			sub.Clear()
			f := g.newFadePlay(sub)
			g.rec.playIn(sub, in, f, r)
			f.done()
		}
		s.stats.shifts++
	}
	s.overlayOrigin = origin
	real.DrawImage(s.overlay, nil)
}

// scratchHold is how many frames the pan scratch is kept after the last
// shift through it (item 162): a frame-sized texture, 16 MiB at
// 1920×1200, that a city at rest has no use for. Frames rather than
// seconds on purpose, as the look cache counts them.
const scratchHold = 300

// restScratch counts a frame towards letting the scratch go; a shift
// through it starts the count again.
func (g *Game) restScratch() {
	s := &g.still
	if s.scratch == nil {
		return
	}
	s.unshifted++
	if s.unshifted >= scratchHold {
		s.scratch.Deallocate()
		s.scratch = nil
	}
}

// shiftLayer moves *img by k through the scratch image, which then
// takes *img's old pixels' place as the scratch. What the move uncovers
// keeps whatever the scratch held, for the caller to compose.
func (g *Game) shiftLayer(img **ebiten.Image, k image.Point) {
	s := &g.still
	s.unshifted = 0
	s.scratch = sizedImage(s.scratch, (*img).Bounds().Size())
	op := &ebiten.DrawImageOptions{Blend: ebiten.BlendCopy}
	op.GeoM.Translate(float64(k.X), float64(k.Y))
	s.scratch.DrawImage(*img, op)
	*img, s.scratch = s.scratch, *img
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
