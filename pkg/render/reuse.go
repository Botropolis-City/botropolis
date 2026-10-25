package render

import (
	"encoding/binary"
	"hash/maphash"
	"image"
	"log"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// Kept records (item 212). A still drawable whose key holds keeps last
// frame's record, moved by the frame's whole-pixel pan, instead of
// being drawn again only to find its record unchanged. A key is the
// drawable's kind, the values it is drawn from and what it reads of the
// frame. Every verifyEvery-th kept record is made afresh as well and
// compared with the one kept: a difference is a key that misses an
// input, which a debug build fails on and a release corrects and logs.

// recordKind is a kind of drawable that can keep its record.
type recordKind uint8

const (
	kindNone recordKind = iota
	kindDistrictFloor
	kindLotFloor
)

var recordKindNames = [...]string{"", "a district's aged floor", "a building's aged lot"}

func (k recordKind) String() string { return recordKindNames[k] }

// releaseVerifyEvery is how often a release makes a kept record afresh
// to check it: one in 16, so a key that misses an input shows a stale
// city for a moment, not until the next change (Aria, 2026-10-25).
const releaseVerifyEvery = 16

// keptRecord is last frame's record, kept for this frame to take from.
type keptRecord struct {
	cmds     []drawCmd
	vertices []ebiten.Vertex
	indices  []uint32
	points   []city.Point
	entries  []entry
	// at is each keyed entry's place in entries, by key.
	at     map[uint64]int
	origin city.Point
	// held is whether the record was put aside for a frame drawn
	// straight, which records nothing to keep, so the next frame keeps
	// from the last one that did.
	held bool
}

// noted is what a drawable did besides drawing, counted: a record is
// kept only when what it did can be done again from the count.
type noted struct {
	hits, lightOps, litRects, cutPoints, cutIndices, glows int
	windows, landmarks, punches, buildings                 int
	world                                                  int
	fade, fades                                            int
}

func (g *Game) noted() noted {
	l := g.rec
	return noted{
		hits: len(g.frameHits), lightOps: len(g.lightOps), litRects: len(g.litRects),
		cutPoints: len(g.cutPoints), cutIndices: len(g.cutIndices), glows: len(g.glows),
		windows: g.windowDraws, landmarks: g.landmarkDraws, punches: g.punches, buildings: g.buildingsDrawn,
		world: g.drawing.n, fade: l.fade, fades: l.fades,
	}
}

// keepRecord puts last frame's record aside for this one to take from,
// and gives the list the buffers of the one before to record into. A
// frame drawn straight keeps from nothing and leaves aside what it
// found, as the frame check draws each frame straight and then not.
func (g *Game) keepRecord() {
	l, k := g.recorder(), &g.still.kept
	if k.held {
		k.held = g.noStill
		return
	}
	k.held = g.noStill
	k.cmds, l.cmds = l.cmds, k.cmds
	k.vertices, l.vertices = l.vertices, k.vertices
	k.indices, l.indices = l.indices, k.indices
	k.points, l.points = l.points, k.points
	k.entries, g.still.entries = g.still.entries, k.entries
	k.origin = l.origin
	if k.at == nil {
		k.at = map[uint64]int{}
	}
	clear(k.at)
	for i, e := range k.entries {
		if e.key != 0 && e.keeps {
			k.at[e.key] = i
		}
	}
}

// noteKeyed runs a still drawable of kind as note does, keyed by key:
// the values it is drawn from. When last frame recorded the same, it
// keeps that record instead.
func (g *Game) noteKeyed(kind recordKind, key uint64, draw func()) {
	if g.noStill || g.noReuse {
		g.note(draw, false)
		return
	}
	l := g.rec
	key = g.frameKey(kind, key)
	k := &g.still.kept
	i, found := k.at[key]
	var shift image.Point
	if found && k.entries[i].to > len(k.cmds) {
		// Never past the commands of the frame it was recorded in: a
		// slice within the buffer's capacity reads what an older frame
		// left there.
		found = false
	}
	if found {
		shift, found = pixelShift(k.origin, l.origin)
	}
	if !found {
		g.noteFresh(key, draw)
		g.still.stats.fresh++
		return
	}
	g.verifyCount++
	if g.verifyEvery <= 0 || g.verifyCount%g.verifyEvery != 0 {
		g.keep(k, i, shift)
		g.still.stats.reused++
		return
	}
	// The record made afresh stands; the kept one is made beside it,
	// compared, and let go, so the check covers keeping it as well as
	// its key.
	g.noteFresh(key, draw)
	fresh := len(g.still.entries) - 1
	mark, drawn := l.mark(), g.drawing
	g.keep(k, i, shift)
	same := g.sameRecord(&g.still.entries[len(g.still.entries)-1], &g.still.entries[fresh])
	l.truncate(mark)
	g.still.entries = g.still.entries[:fresh+1]
	g.drawing.n, g.drawing.lastWorld = drawn.n, drawn.lastWorld
	if same {
		g.still.stats.reused++
		return
	}
	g.still.stats.stale++
	g.staleKey(kind)
}

// noteFresh records a drawable afresh under key, saying whether its
// record can be kept: it is still, and all it did besides drawing was
// count world draws.
func (g *Game) noteFresh(key uint64, draw func()) {
	before := g.noted()
	g.note(draw, false)
	after := g.noted()
	e := &g.still.entries[len(g.still.entries)-1]
	e.key, e.world = key, after.world-before.world
	after.world = before.world
	e.keeps = !e.moves && !e.active && after == before
}

// keep copies kept entry i into this frame's record, moved by shift,
// and counts its world draws again.
func (g *Game) keep(k *keptRecord, i int, shift image.Point) {
	l := g.rec
	e := k.entries[i]
	dx, dy := float32(shift.X), float32(shift.Y)
	from := len(l.cmds)
	for _, c := range k.cmds[e.from:e.to] {
		c.box = moved(c.box, shift)
		switch c.kind {
		case cmdImage:
			c.img.GeoM.Translate(float64(dx), float64(dy))
		case cmdColorM:
			c.cmImg.GeoM.Translate(float64(dx), float64(dy))
		case cmdWords:
			c.text.GeoM.Translate(float64(dx), float64(dy))
		case cmdMesh:
			v := len(l.vertices)
			for _, x := range k.vertices[c.v[0]:c.v[1]] {
				x.DstX += dx
				x.DstY += dy
				l.vertices = append(l.vertices, x)
			}
			n := len(l.indices)
			l.indices = append(l.indices, k.indices[c.i[0]:c.i[1]]...)
			c.v, c.i = [2]int{v, len(l.vertices)}, [2]int{n, len(l.indices)}
		case cmdRing:
			p := len(l.points)
			for _, x := range k.points[c.p[0]:c.p[1]] {
				l.points = append(l.points, x.Add(city.Point{X: float64(dx), Y: float64(dy)}))
			}
			c.p = [2]int{p, len(l.points)}
		}
		l.cmds = append(l.cmds, c)
	}
	for range e.world {
		g.drawing.world()
	}
	n := len(g.lightOps)
	g.still.entries = append(g.still.entries, entry{
		inked:  inked{from: from, to: len(l.cmds), box: moved(e.box, shift), hash: e.hash},
		lights: [2]int{n, n}, key: e.key, world: e.world, keeps: true,
	})
}

// sameRecord is whether kept, an entry kept this frame, is fresh, the
// same drawable recorded afresh: the same draws of the same sources,
// inking the same boxes and doing the same besides, to within
// keepTolerance of a pixel.
func (g *Game) sameRecord(kept, fresh *entry) bool {
	if !fresh.keeps || fresh.hash != kept.hash || fresh.box != kept.box || fresh.world != kept.world || fresh.to-fresh.from != kept.to-kept.from {
		return false
	}
	for j := range kept.to - kept.from {
		if !g.rec.sameCmd(&g.rec.cmds[kept.from+j], &g.rec.cmds[fresh.from+j]) {
			return false
		}
	}
	return true
}

// keepTolerance is how far apart, in pixels, a kept draw and the same
// draw made afresh may land: a pan moves the kept one by a whole number
// added to its floats, which the one worked out afresh rounds otherwise.
const keepTolerance = 1.0 / 64

// sameCmd is whether two recorded draws draw the same.
func (l *drawList) sameCmd(a, b *drawCmd) bool {
	if a.kind != b.kind || a.hash != b.hash || a.box != b.box || a.fade != b.fade || a.src != b.src || a.page != b.page {
		return false
	}
	near := func(x, y float64) bool { return math.Abs(x-y) <= keepTolerance }
	geo := func(x, y ebiten.GeoM) bool {
		for i := range 2 {
			for j := range 3 {
				if !near(x.Element(i, j), y.Element(i, j)) {
					return false
				}
			}
		}
		return true
	}
	switch a.kind {
	case cmdImage:
		return geo(a.img.GeoM, b.img.GeoM) && a.img.ColorScale == b.img.ColorScale
	case cmdColorM:
		return geo(a.cmImg.GeoM, b.cmImg.GeoM)
	case cmdWords:
		return geo(a.text.GeoM, b.text.GeoM) && a.words == b.words
	case cmdMesh:
		if a.v[1]-a.v[0] != b.v[1]-b.v[0] || a.i[1]-a.i[0] != b.i[1]-b.i[0] {
			return false
		}
		for j := range a.v[1] - a.v[0] {
			x, y := l.vertices[a.v[0]+j], l.vertices[b.v[0]+j]
			if !near(float64(x.DstX), float64(y.DstX)) || !near(float64(x.DstY), float64(y.DstY)) ||
				x.SrcX != y.SrcX || x.SrcY != y.SrcY || x.ColorR != y.ColorR || x.ColorG != y.ColorG || x.ColorB != y.ColorB || x.ColorA != y.ColorA {
				return false
			}
		}
		for j := range a.i[1] - a.i[0] {
			if l.indices[a.i[0]+j] != l.indices[b.i[0]+j] {
				return false
			}
		}
	case cmdRing:
		if a.p[1]-a.p[0] != b.p[1]-b.p[0] {
			return false
		}
		for j := range a.p[1] - a.p[0] {
			x, y := l.points[a.p[0]+j], l.points[b.p[0]+j]
			if !near(x.X, y.X) || !near(x.Y, y.Y) {
				return false
			}
		}
	}
	return true
}

// staleKey is a kept record that differs from the one made afresh: a
// key missing an input. The record made afresh stands.
func (g *Game) staleKey(kind recordKind) {
	if failOnStaleKey {
		panic("item 212: a kept record of " + kind.String() + " differs from the one made afresh: its key misses an input")
	}
	if g.staleLogged == nil {
		g.staleLogged = map[recordKind]bool{}
	}
	if !g.staleLogged[kind] {
		g.staleLogged[kind] = true
		log.Printf("botropolis: a kept record of %s differed from the one made afresh (item 212)", kind)
	}
}

// moved is r moved by shift; an empty box inks nowhere, wherever the
// frame moves.
func moved(r image.Rectangle, shift image.Point) image.Rectangle {
	if r.Empty() {
		return r
	}
	return r.Add(shift)
}

// pixelShift is how far, in whole pixels, the frame moved from one
// origin to another, and whether it moved by whole pixels at all.
func pixelShift(from, to city.Point) (image.Point, bool) {
	dx, dy := to.X-from.X, to.Y-from.Y
	kx, ky := math.Round(dx), math.Round(dy)
	if math.Abs(dx-kx) > 1e-6 || math.Abs(dy-ky) > 1e-6 {
		return image.Point{}, false
	}
	return image.Pt(int(kx), int(ky)), true
}

// frameKey folds into a drawable's key its kind and what it reads of
// the frame: the camera but for where it is, the kits and the level
// drawn from, reduced motion, whether a view drains the colour from
// it, and the fade group it is recorded into.
func (g *Game) frameKey(kind recordKind, key uint64) uint64 {
	var h maphash.Hash
	h.SetSeed(inkSeed)
	var b [8]byte
	put := func(v uint64) {
		binary.LittleEndian.PutUint64(b[:], v)
		h.Write(b[:])
	}
	cam := g.keyCam
	put(uint64(kind))
	put(key)
	put(math.Float64bits(cam.Zoom))
	put(uint64(cam.Heading))
	put(uint64(cam.Projection))
	put(uint64(g.kitsGeneration()))
	if g.reduced {
		put(1)
	}
	if g.viewing() && !g.faultyKeys {
		put(2)
	}
	put(uint64(g.rec.fade))
	put(uint64(g.rec.fades))
	return h.Sum64() | 1
}

// kitsGeneration counts the times the kits' resident pages changed, so
// a record drawn from a page let go, or from a level drawn in while
// another goes up, is not kept.
func (g *Game) kitsGeneration() int {
	if g.kits == nil {
		return 0
	}
	return g.kits.generation
}
