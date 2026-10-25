package render

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// A drawable's noteLog is what it noted while it drew, besides drawing
// (item 212): the looks it drew from, the words it showed, its glows,
// the night's light ops, and how many it counted of each. A kept record
// does all of it again, moved by the pan, and the verify mode compares
// a kept noteLog with the one made afresh.

// lookUse is a look a drawable drew from: kept only while the cache
// still holds that look under that key.
type lookUse struct {
	key  lookKey
	look *bakedLook
}

// jspan is a range in one of the frame's buffers.
type jspan [2]int

func (s jspan) n() int { return s[1] - s[0] }

// noteLog is an entry's spans of the frame's noteLog buffers, and what
// it counted.
type noteLog struct {
	looks, words, glows jspan
	// lookKeys is the key of every look run it drew, from a look or
	// straight: what its painting was worked out from.
	lookKeys                          jspan
	litRects, cutPoints, cutVertices  jspan
	cutIndices                        jspan
	windows, landmarks, punches, cuts int
	buildings                         int
	// runs, hits and misses are the look runs it drew, and how many
	// came from a look and how many were drawn straight.
	runs, hits, misses int
}

// journalBuffers is what a frame's drawables noted, kept beside the
// game's own buffers for the frame after to take from.
type journalBuffers struct {
	looks       []lookUse
	lookKeys    []lookKey
	words       []string
	glows       []city.Point
	lightOps    []lightOp
	litRects    []image.Rectangle
	cutPoints   []city.Point
	cutVertices []ebiten.Vertex
	cutIndices  []uint32
	// lit is whether the frame laid lights, so its light buffers are its
	// own and not the last night's.
	lit bool
}

// seeText tells the frame's listener a string is shown, and notes it
// in the noteLog of the drawable recorded now.
func (g *Game) seeText(s string) {
	if g.textSeen == nil {
		return
	}
	g.jn.words = append(g.jn.words, s)
	g.textSeen(s)
}

// journalLook notes a look the drawable recorded now draws from.
func (g *Game) journalLook(key lookKey, look *bakedLook) {
	g.jn.looks = append(g.jn.looks, lookUse{key: key, look: look})
}

// unsettle says the drawable recorded now changed a cache, or would
// draw differently with nothing it is keyed by changing, so its record
// is not kept.
func (l *drawList) unsettle() {
	if l != nil && l.open {
		l.unsettled = true
	}
}

// journalMark is how far each noteLog buffer had got.
type journalMark struct {
	looks, words, glows, lightOps, litRects, cutPoints, cutVertices, cutIndices int
	lookKeys                                                                    int
	windows, landmarks, punches, cuts, buildings                                int
	hits, world, fade, fades                                                    int
	runs, lookHits, misses                                                      int
}

func (g *Game) journalMark() journalMark {
	return journalMark{
		looks: len(g.jn.looks), lookKeys: len(g.jn.lookKeys), words: len(g.jn.words), glows: len(g.glows), lightOps: len(g.lightOps),
		litRects: len(g.litRects), cutPoints: len(g.cutPoints), cutVertices: len(g.cutVertices), cutIndices: len(g.cutIndices),
		windows: g.windowDraws, landmarks: g.landmarkDraws, punches: g.punches, cuts: g.cuts, buildings: g.buildingsDrawn,
		hits: len(g.frameHits), world: g.drawing.n, fade: g.rec.fade, fades: g.rec.fades,
		runs: g.lookStats.runs, lookHits: g.lookStats.hits, misses: g.lookStats.misses,
	}
}

// journalSince is what was noted between two marks.
func journalSince(a, b journalMark) noteLog {
	return noteLog{
		looks: jspan{a.looks, b.looks}, lookKeys: jspan{a.lookKeys, b.lookKeys}, words: jspan{a.words, b.words}, glows: jspan{a.glows, b.glows},
		litRects: jspan{a.litRects, b.litRects}, cutPoints: jspan{a.cutPoints, b.cutPoints},
		cutVertices: jspan{a.cutVertices, b.cutVertices}, cutIndices: jspan{a.cutIndices, b.cutIndices},
		windows: b.windows - a.windows, landmarks: b.landmarks - a.landmarks, punches: b.punches - a.punches,
		cuts: b.cuts - a.cuts, buildings: b.buildings - a.buildings,
		runs: b.runs - a.runs, hits: b.lookHits - a.lookHits, misses: b.misses - a.misses,
	}
}

// putJournalAside keeps this frame's noteLog for the next, and starts
// the frame's own.
func (g *Game) putJournalAside(k *journalBuffers) {
	k.looks, g.jn.looks = g.jn.looks, k.looks[:0]
	k.lookKeys, g.jn.lookKeys = g.jn.lookKeys, k.lookKeys[:0]
	k.words, g.jn.words = g.jn.words, k.words[:0]
	k.glows = append(k.glows[:0], g.glows...)
	k.lit = g.litFrame
	if g.litFrame {
		k.lightOps = append(k.lightOps[:0], g.lightOps...)
		k.litRects = append(k.litRects[:0], g.litRects...)
		k.cutPoints = append(k.cutPoints[:0], g.cutPoints...)
		k.cutVertices = append(k.cutVertices[:0], g.cutVertices...)
		k.cutIndices = append(k.cutIndices[:0], g.cutIndices...)
	}
	g.litFrame = false
}

// looksHeld is whether the cache still holds every look a kept noteLog
// drew from, as the same look.
func (g *Game) looksHeld(k *journalBuffers, j noteLog) bool {
	for _, u := range k.looks[j.looks[0]:j.looks[1]] {
		if g.looks == nil || g.lookPages == nil {
			return false
		}
		look, ok := g.looks.peek(u.key)
		if !ok || look != u.look {
			return false
		}
	}
	return true
}

// replay does again what a kept noteLog noted, moved by shift, and is
// the noteLog of the entry it is kept for.
func (g *Game) replay(k *journalBuffers, e *entry, shift image.Point) (noteLog, [2]int) {
	from := g.journalMark()
	j := e.jn
	for _, u := range k.looks[j.looks[0]:j.looks[1]] {
		look, _ := g.looks.get(u.key)
		if look.paged {
			g.lookPages.use(look.onPage, g.lookFrame)
		}
		g.jn.looks = append(g.jn.looks, u)
	}
	g.jn.lookKeys = append(g.jn.lookKeys, k.lookKeys[j.lookKeys[0]:j.lookKeys[1]]...)
	for _, w := range k.words[j.words[0]:j.words[1]] {
		g.seeText(w)
	}
	d := city.Point{X: float64(shift.X), Y: float64(shift.Y)}
	for _, p := range k.glows[j.glows[0]:j.glows[1]] {
		g.glows = append(g.glows, p.Add(d))
	}
	g.windowDraws += j.windows
	g.landmarkDraws += j.landmarks
	g.punches += j.punches
	g.cuts += j.cuts
	g.buildingsDrawn += j.buildings
	g.lookStats.runs += j.runs
	g.lookStats.hits += j.hits
	g.lookStats.misses += j.misses
	lights := [2]int{len(g.lightOps), len(g.lightOps)}
	if e.lights[1] > e.lights[0] {
		g.replayLights(k, e, shift)
		lights[1] = len(g.lightOps)
	}
	return journalSince(from, g.journalMark()), lights
}

// replayLights lays a kept entry's light ops again, moved by shift,
// with the panes, points, vertices and indices they refer to.
func (g *Game) replayLights(k *journalBuffers, e *entry, shift image.Point) {
	j := e.jn
	dx, dy := float64(shift.X), float64(shift.Y)
	litBase := len(g.litRects) - j.litRects[0]
	ptBase := len(g.cutPoints) - j.cutPoints[0]
	idxBase := len(g.cutIndices) - j.cutIndices[0]
	vBase := uint32(len(g.cutVertices) - j.cutVertices[0])
	for _, r := range k.litRects[j.litRects[0]:j.litRects[1]] {
		r = moved(r, shift)
		g.litRects = append(g.litRects, r)
		g.litBounds = g.litBounds.Union(r)
		g.foldLit(r)
	}
	for _, p := range k.cutPoints[j.cutPoints[0]:j.cutPoints[1]] {
		g.cutPoints = append(g.cutPoints, p.Add(city.Point{X: dx, Y: dy}))
	}
	for _, v := range k.cutVertices[j.cutVertices[0]:j.cutVertices[1]] {
		v.DstX += float32(dx)
		v.DstY += float32(dy)
		g.cutVertices = append(g.cutVertices, v)
	}
	for _, i := range k.cutIndices[j.cutIndices[0]:j.cutIndices[1]] {
		g.cutIndices = append(g.cutIndices, i+vBase)
	}
	for _, op := range k.lightOps[e.lights[0]:e.lights[1]] {
		op.box = moved(op.box, shift)
		if op.placed() {
			op.geo.Translate(dx, dy)
		}
		switch {
		case op.punch:
		case op.cut == cutRing:
			op.from, op.to = op.from+ptBase, op.to+ptBase
		case op.cut == cutMesh:
			op.from, op.to = op.from+idxBase, op.to+idxBase
		case op.cut == cutNone:
			op.from, op.to = op.from+litBase, op.to+litBase
			vs := make([]ebiten.Vertex, len(op.vertices))
			for i, v := range op.vertices {
				v.DstX += float32(dx)
				v.DstY += float32(dy)
				vs[i] = v
			}
			op.vertices = vs
		}
		g.lightOps = append(g.lightOps, op)
	}
	g.meshCut = -1
}

// sameJournal is whether a kept noteLog, moved by shift, is the one made
// afresh: the same looks, words, glows, counts and light ops.
func (g *Game) sameJournal(k *journalBuffers, kept *entry, shift image.Point, fresh *entry) bool {
	a, b := kept.jn, fresh.jn
	if a.looks.n() != b.looks.n() || a.words.n() != b.words.n() || a.glows.n() != b.glows.n() ||
		a.litRects.n() != b.litRects.n() || a.cutPoints.n() != b.cutPoints.n() || a.cutVertices.n() != b.cutVertices.n() ||
		a.cutIndices.n() != b.cutIndices.n() || a.windows != b.windows || a.landmarks != b.landmarks ||
		a.punches != b.punches || a.cuts != b.cuts || a.buildings != b.buildings ||
		a.runs != b.runs || a.hits != b.hits || a.misses != b.misses ||
		kept.lights[1]-kept.lights[0] != fresh.lights[1]-fresh.lights[0] {
		return false
	}
	if !g.sameLookKeys(k, a, b) {
		return false
	}
	near := func(x, y float64) bool { return x-y <= keepTolerance && y-x <= keepTolerance }
	d := city.Point{X: float64(shift.X), Y: float64(shift.Y)}
	for i := range a.looks.n() {
		if k.looks[a.looks[0]+i] != g.jn.looks[b.looks[0]+i] {
			return false
		}
	}
	for i := range a.words.n() {
		if k.words[a.words[0]+i] != g.jn.words[b.words[0]+i] {
			return false
		}
	}
	for i := range a.glows.n() {
		p, q := k.glows[a.glows[0]+i].Add(d), g.glows[b.glows[0]+i]
		if !near(p.X, q.X) || !near(p.Y, q.Y) {
			return false
		}
	}
	for i := range a.litRects.n() {
		if !nearBox(moved(k.litRects[a.litRects[0]+i], shift), g.litRects[b.litRects[0]+i]) {
			return false
		}
	}
	for i := range a.cutPoints.n() {
		p, q := k.cutPoints[a.cutPoints[0]+i].Add(d), g.cutPoints[b.cutPoints[0]+i]
		if !near(p.X, q.X) || !near(p.Y, q.Y) {
			return false
		}
	}
	for i := range a.cutVertices.n() {
		v, w := k.cutVertices[a.cutVertices[0]+i], g.cutVertices[b.cutVertices[0]+i]
		if !near(float64(v.DstX)+d.X, float64(w.DstX)) || !near(float64(v.DstY)+d.Y, float64(w.DstY)) {
			return false
		}
	}
	for i := range a.cutIndices.n() {
		if int(k.cutIndices[a.cutIndices[0]+i])-a.cutVertices[0] != int(g.cutIndices[b.cutIndices[0]+i])-b.cutVertices[0] {
			return false
		}
	}
	for i := range kept.lights[1] - kept.lights[0] {
		x, y := k.lightOps[kept.lights[0]+i], g.lightOps[fresh.lights[0]+i]
		if x.punch != y.punch || x.cut != y.cut || !sameSource(&drawCmd{src: x.src, page: x.page}, &drawCmd{src: y.src, page: y.page}) || x.page != y.page || x.warm != y.warm ||
			x.alpha != y.alpha || x.words != y.words || x.face != y.face || !nearBox(moved(x.box, shift), y.box) ||
			x.to-x.from != y.to-y.from || len(x.vertices) != len(y.vertices) || len(x.indices) != len(y.indices) {
			return false
		}
		if x.placed() {
			x.geo.Translate(d.X, d.Y)
		}
		for r := range 2 {
			for c := range 3 {
				if !near(x.geo.Element(r, c), y.geo.Element(r, c)) {
					return false
				}
			}
		}
		for v := range x.vertices {
			p, q := x.vertices[v], y.vertices[v]
			if !near(float64(p.DstX)+d.X, float64(q.DstX)) || !near(float64(p.DstY)+d.Y, float64(q.DstY)) ||
				p.SrcX != q.SrcX || p.SrcY != q.SrcY || p.ColorA != q.ColorA {
				return false
			}
		}
		for v := range x.indices {
			if x.indices[v] != y.indices[v] {
				return false
			}
		}
	}
	return true
}

// sameKept is whether a kept record, moved by shift, is the drawable as
// made afresh. One made afresh that is unsettled — it baked a look, drew
// one straight whose key is new, or drew from one baked off the frame —
// draws the same in another form, a look for steps or steps for a look,
// so it is compared by what it was worked out from: its looks' keys, and
// what no look holds, its lit panes, its glows and its windows.
func (g *Game) sameKept(k *journalBuffers, was, kept *entry, shift image.Point, fresh *entry) bool {
	if fresh.keeps {
		return g.sameRecord(kept, fresh) && g.sameJournal(k, was, shift, fresh)
	}
	a, b := was.jn, fresh.jn
	if !g.sameLookKeys(k, a, b) || a.litRects.n() != b.litRects.n() || a.glows.n() != b.glows.n() || a.windows != b.windows {
		return false
	}
	d := city.Point{X: float64(shift.X), Y: float64(shift.Y)}
	for i := range a.litRects.n() {
		if !nearBox(moved(k.litRects[a.litRects[0]+i], shift), g.litRects[b.litRects[0]+i]) {
			return false
		}
	}
	for i := range a.glows.n() {
		p, q := k.glows[a.glows[0]+i].Add(d), g.glows[b.glows[0]+i]
		if p.X-q.X > keepTolerance || q.X-p.X > keepTolerance || p.Y-q.Y > keepTolerance || q.Y-p.Y > keepTolerance {
			return false
		}
	}
	return true
}

// sameLookKeys is whether two journals drew their look runs under the
// same keys.
func (g *Game) sameLookKeys(k *journalBuffers, a, b noteLog) bool {
	if a.lookKeys.n() != b.lookKeys.n() {
		return false
	}
	for i := range a.lookKeys.n() {
		if k.lookKeys[a.lookKeys[0]+i] != g.jn.lookKeys[b.lookKeys[0]+i] {
			return false
		}
	}
	return true
}

// placed is whether a light op is put where its transform says: a
// punch, a stamp's cut or a word's. A window's panes and a ring's or a
// mesh's points carry their own places.
func (op lightOp) placed() bool {
	return op.punch || op.cut == cutStamp || op.cut == cutWords
}

// foldLit folds a lit pane into the running hash of the panes lit so
// far this frame, relative to the camera's whole-pixel origin: whether a
// sprite cuts the lights depends on the windows lit before it, which a
// drawable's own key cannot see.
func (g *Game) foldLit(r image.Rectangle) {
	var ox, oy int
	if g.rec != nil {
		ox, oy = int(math.Floor(g.rec.origin.X)), int(math.Floor(g.rec.origin.Y))
	}
	h := g.litHash
	for _, v := range [4]int{r.Min.X - ox, r.Min.Y - oy, r.Max.X - ox, r.Max.Y - oy} {
		h ^= uint64(int64(v))
		h *= 0x100000001b3
	}
	g.litHash = h
}
