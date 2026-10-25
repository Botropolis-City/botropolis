package render

import (
	"encoding/binary"
	"hash/maphash"
	"image"
	"log"
	"math"
	"unsafe"

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
	kindHouse
	kindMonument
	kindSign
	kindTree
	kindBridge
	kindsKept
)

var recordKindNames = [kindsKept]string{"", "a district's aged floor", "a building's aged lot", "a building",
	"a district's monument", "a district's sign", "a tree", "a piece of the bridge"}

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
	jn       journalBuffers
	// at is each keyed entry's place in entries, by key.
	at     map[uint64]int
	origin city.Point
	// held is whether the record was put aside for a frame drawn
	// straight, which records nothing to keep, so the next frame keeps
	// from the last one that did.
	held bool
}

// keepRecord puts last frame's record aside for this one to take from,
// and gives the list the buffers of the one before to record into. A
// frame drawn straight keeps from nothing and leaves aside what it
// found, as the frame check draws each frame straight and then not.
func (g *Game) keepRecord() {
	l, k := g.recorder(), &g.still.kept
	if k.held {
		k.held = g.noStill
		g.jn.looks, g.jn.lookKeys, g.jn.words = g.jn.looks[:0], g.jn.lookKeys[:0], g.jn.words[:0]
		g.litFrame = false
		return
	}
	k.held = g.noStill
	k.cmds, l.cmds = l.cmds, k.cmds
	k.vertices, l.vertices = l.vertices, k.vertices
	k.indices, l.indices = l.indices, k.indices
	k.points, l.points = l.points, k.points
	k.entries, g.still.entries = g.still.entries, k.entries
	g.putJournalAside(&k.jn)
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
	if found && !k.within(k.entries[i]) {
		// Never past what the frame it was recorded in left: a slice
		// within a buffer's capacity reads what an older frame left.
		found = false
	}
	if found {
		shift, found = pixelShift(k.origin, l.origin)
	}
	if found {
		found = g.looksHeld(&k.jn, k.entries[i].jn)
	}
	if !found {
		g.noteFresh(key, draw)
		g.still.stats.fresh++
		g.still.stats.freshBy[kind]++
		return
	}
	g.verifyCount++
	if g.verifyEvery <= 0 || g.verifyCount%g.verifyEvery != 0 {
		g.keep(k, i, shift)
		g.still.stats.reused++
		g.still.stats.keptBy[kind]++
		return
	}
	// The record made afresh stands; the kept one is made beside it,
	// compared, and let go, so the check covers keeping it as well as
	// its key. Its noteLog is compared where it was kept, not done again.
	g.noteFresh(key, draw)
	fresh := len(g.still.entries) - 1
	mark := l.mark()
	g.copyRecord(k, i, shift)
	kept := &g.still.entries[len(g.still.entries)-1]
	same := g.sameKept(&k.jn, &k.entries[i], kept, shift, &g.still.entries[fresh])
	l.truncate(mark)
	g.still.entries = g.still.entries[:fresh+1]
	if same {
		g.still.stats.reused++
		g.still.stats.keptBy[kind]++
		return
	}
	g.still.stats.stale++
	g.staleKey(kind)
}

// within is whether a kept entry's spans all lie in what its own frame
// recorded and noted.
func (k *keptRecord) within(e entry) bool {
	if e.to > len(k.cmds) || e.jn.looks[1] > len(k.jn.looks) || e.jn.lookKeys[1] > len(k.jn.lookKeys) || e.jn.words[1] > len(k.jn.words) || e.jn.glows[1] > len(k.jn.glows) {
		return false
	}
	if e.lights[1] == e.lights[0] {
		return true
	}
	return k.jn.lit && e.lights[1] <= len(k.jn.lightOps) && e.jn.litRects[1] <= len(k.jn.litRects) &&
		e.jn.cutPoints[1] <= len(k.jn.cutPoints) && e.jn.cutVertices[1] <= len(k.jn.cutVertices) && e.jn.cutIndices[1] <= len(k.jn.cutIndices)
}

// noteFresh records a drawable afresh under key, with its noteLog,
// saying whether its record can be kept: it is still and settled, and
// noted no hit and began no fade group.
func (g *Game) noteFresh(key uint64, draw func()) {
	before := g.journalMark()
	g.note(draw, false)
	after := g.journalMark()
	e := &g.still.entries[len(g.still.entries)-1]
	e.key, e.world, e.jn = key, after.world-before.world, journalSince(before, after)
	e.keeps = !e.moves && !e.active && !g.rec.unsettled && after.hits == before.hits &&
		after.fade == before.fade && after.fades == before.fades
}

// keep copies kept entry i into this frame's record, moved by shift,
// and does again what it noted.
func (g *Game) keep(k *keptRecord, i int, shift image.Point) {
	g.copyRecord(k, i, shift)
	e := &g.still.entries[len(g.still.entries)-1]
	for range e.world {
		g.drawing.world()
	}
	e.jn, e.lights = g.replay(&k.jn, &k.entries[i], shift)
}

// copyRecord copies kept entry i's draws into this frame's record,
// moved by shift, as an entry of its own.
func (g *Game) copyRecord(k *keptRecord, i int, shift image.Point) {
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
	n := len(g.lightOps)
	g.still.entries = append(g.still.entries, entry{
		inked:  inked{from: from, to: len(l.cmds), box: moved(e.box, shift), hash: e.hash},
		lights: [2]int{n, n}, key: e.key, world: e.world, jn: e.jn, keeps: true,
	})
}

// sameRecord is whether kept, an entry kept this frame, is fresh, the
// same drawable recorded afresh: the same draws of the same sources,
// to within keepTolerance of a pixel, inking boxes within a pixel, and
// doing the same besides. Hashes are not compared: they round positions
// to a 256th of a pixel, so one on a boundary hashes either way.
func (g *Game) sameRecord(kept, fresh *entry) bool {
	if !fresh.keeps || !nearBox(fresh.box, kept.box) || fresh.world != kept.world || fresh.to-fresh.from != kept.to-kept.from {
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

// nearBox is whether two boxes are within a pixel at every edge: a box
// rounds out to whole pixels, so an edge within rounding of one lands
// either side of it.
func nearBox(a, b image.Rectangle) bool {
	near := func(x, y int) bool { return x-y <= 1 && y-x <= 1 }
	return a.Empty() == b.Empty() && near(a.Min.X, b.Min.X) && near(a.Min.Y, b.Min.Y) && near(a.Max.X, b.Max.X) && near(a.Max.Y, b.Max.Y)
}

// sameImageOptions is whether two draws of an image draw it alike, but
// for where.
func sameImageOptions(a, b *ebiten.DrawImageOptions) bool {
	return a.ColorScale == b.ColorScale && a.Blend == b.Blend && a.Filter == b.Filter && a.DisableMipmaps == b.DisableMipmaps
}

// sameTriangleOptions is whether two meshes are drawn alike.
func sameTriangleOptions(a, b *ebiten.DrawTrianglesOptions) bool {
	return a.ColorScaleMode == b.ColorScaleMode && a.Blend == b.Blend && a.Filter == b.Filter && a.Address == b.Address &&
		a.DisableMipmaps == b.DisableMipmaps
}

// sameCmd is whether two recorded draws draw the same.
func (l *drawList) sameCmd(a, b *drawCmd) bool {
	if a.kind != b.kind || !nearBox(a.box, b.box) || a.fade != b.fade || !sameSource(a, b) || a.page != b.page {
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
		return geo(a.img.GeoM, b.img.GeoM) && sameImageOptions(&a.img, &b.img)
	case cmdColorM:
		x, y := a.cmImg, b.cmImg
		x.GeoM, y.GeoM = ebiten.GeoM{}, ebiten.GeoM{}
		for i := range 4 {
			for j := range 5 {
				if a.cm.Element(i, j) != b.cm.Element(i, j) {
					return false
				}
			}
		}
		return geo(a.cmImg.GeoM, b.cmImg.GeoM) && x == y
	case cmdWords:
		return geo(a.text.GeoM, b.text.GeoM) && sameImageOptions(&a.text.DrawImageOptions, &b.text.DrawImageOptions) &&
			a.text.LayoutOptions == b.text.LayoutOptions && a.words == b.words && a.face == b.face
	case cmdFill:
		return a.fill == b.fill
	case cmdMesh:
		if !sameTriangleOptions(&a.tri, &b.tri) || a.v[1]-a.v[0] != b.v[1]-b.v[0] || a.i[1]-a.i[0] != b.i[1]-b.i[0] {
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
		if a.path != b.path || a.p[1]-a.p[0] != b.p[1]-b.p[0] {
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
// it, the fade group it is recorded into, at night the windows lit
// before it, and for a building the look cache and its pages.
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
	if g.lightsOn {
		// What a sprite cuts of the lights depends on the windows lit
		// before it, which are other drawables'.
		put(3)
		put(g.litHash)
	}
	if kind == kindHouse {
		// A look drawn straight for want of room is drawn afresh once a
		// page starts again; a new cache holds none of the looks drawn.
		put(uint64(uintptr(unsafe.Pointer(g.looks))))
		put(uint64(uintptr(unsafe.Pointer(g.lookPages))))
		if g.lookPages != nil {
			put(uint64(g.lookPages.generation))
		}
		if g.noLookCache {
			put(4)
		}
	}
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
