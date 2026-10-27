package render

import (
	"hash/maphash"
	"image"
	"math"
	"reflect"
	"unsafe"

	"botropolis.city/botropolis/pkg/city"
)

// standingKey is what every still drawable reads alike, and each still
// thing's values that change without a snapshot, folded once a frame:
// while it holds from one frame to the next, nothing still on the map
// would be drawn otherwise (item 212). TestStillKeyReads holds every
// still drawable's key and draw to what this reads.
func (g *Game) standingKey(selected *city.Building) uint64 {
	s := g.scene
	if !g.frameBase.ok {
		g.frameBase = g.frameBaseKey()
	}
	c := s.City()
	now := s.Now()
	key := keyMix(g.frameBase.all, g.frameBase.house, g.houseWide, g.houseWideKey(),
		uint64(s.Generation()), uint64(s.GroundGeneration()), uint64(s.View()),
		boolBits(c.Night), boolBits(g.lightsOn), boolBits(g.labelsVisible()), boolBits(s.Scenery()),
		boolBits(s.Detailed()), math.Float64bits(s.PlantLights()), uint64(uintptr(unsafe.Pointer(selected))))
	// Moved by part of a pixel, nothing drawn lands where it did.
	if g.rec != nil {
		o := g.rec.origin
		frac := func(v float64) uint64 { return uint64(math.Round((v-math.Floor(v))*64)) % 64 }
		key = keyMix(key, frac(o.X), frac(o.Y))
	}
	var stirring uint64
	for id := range g.stirring {
		stirring ^= maphash.String(inkSeed, id)
	}
	key = keyMix(key, stirring)
	viewing := g.viewing()
	for _, b := range c.Buildings() {
		id := b.Session.ID
		v := keyMix(boolBits(s.Dimmed(b)), boolBits(g.unlit(b)), math.Float64bits(s.Scaffold(b)),
			boolBits(courseLit(&b.Session, now)), math.Float64bits(s.Rising(id)), math.Float64bits(s.Lights(id)),
			math.Float64bits(s.Mowing(id)))
		if site, ok := s.Construction(id); ok {
			v = keyMix(v, hashOf(&site))
		}
		if viewing {
			if tint, ok := g.viewTint(b); ok {
				v = keyMix(v, 1, uint64(tint.R)<<24|uint64(tint.G)<<16|uint64(tint.B)<<8|uint64(tint.A))
			}
		}
		for i := range min(b.Flags, 3) {
			v = keyMix(v, math.Float64bits(g.flagHoist(b, i)))
		}
		key = keyMix(key, v)
	}
	return key
}

// heldItem is a still thing as a frame noted it: who it is, the entry its
// record became (-1 when it was not one to keep), its kind, and the hits
// and labels it noted, for a frame whose standing key holds to note
// again without running its draw (item 212).
type heldItem struct {
	id     uint64
	entry  int
	kind   recordKind
	hits   []spriteHit
	labels []pendingLabel
	// held is whether this frame noted it as the last did.
	held bool
}

// itemID is who a keyed item is: where it stands, how high, and the code
// that draws it, so two things on one footprint, such as the plant and
// its stack, are told apart.
func itemID(d drawable) uint64 {
	return keyMix(hashOf(&d.turned), uint64(d.level), uint64(reflect.ValueOf(d.keyed).Pointer()))
}

// holding is whether this frame notes its still things as the last did:
// its standing key is the last frame's, the last frame logged them all,
// the camera has moved by whole pixels if at all, and nothing draws them
// otherwise this frame.
func (g *Game) holding(fade *fader) (image.Point, bool) {
	s := &g.still
	if !s.logged || s.standing != s.lastStanding || fade != nil || g.noKeyed || g.noStill || g.noReuse {
		return image.Point{}, false
	}
	return pixelShift(s.kept.origin, g.rec.origin)
}

// noteItem notes keyed item d, the nth of the frame: as the last frame
// noted it while hold lasts and it arrives where the last frame had it,
// otherwise by running it. Arriving anywhere else ends hold for the rest
// of the frame, counted.
func (g *Game) noteItem(d drawable, n int, hold *bool, shift image.Point) {
	s := &g.still
	id := itemID(d)
	if *hold {
		if n >= len(s.lastLog) || s.lastLog[n].id != id {
			*hold = false
			s.stats.orderBroke++
		} else if !d.runs && g.holdItem(&s.lastLog[n], shift) {
			return
		}
	}
	g.mu.Lock()
	hits := len(g.frameHits)
	g.mu.Unlock()
	labels, entries := len(g.mapLabels), len(g.still.entries)
	s.notedKind = kindsKept
	d.keyed()
	it := heldItem{id: id, entry: -1, kind: s.notedKind}
	if len(s.entries) == entries+1 && s.entries[entries].keeps && s.notedKind < kindsKept {
		it.entry = entries
		s.stats.ranKept++
	}
	g.mu.Lock()
	it.hits = append(it.hits, g.frameHits[hits:]...)
	g.mu.Unlock()
	it.labels = append(it.labels, g.mapLabels[labels:]...)
	s.log = append(s.log, it)
	s.stats.ran++
}

// holdItem notes a still thing as the last frame did, moved by shift:
// its kept record, and its hits and labels again. It does not when its
// record was not kept, when the net checks it this frame, when its
// looks have gone, or when it queued labels and the camera has moved,
// since a label is placed where it was queued.
func (g *Game) holdItem(it *heldItem, shift image.Point) bool {
	s := &g.still
	k := &s.kept
	if it.entry < 0 || it.entry >= len(k.entries) || len(it.labels) > 0 && shift != (image.Point{}) {
		return false
	}
	e := k.entries[it.entry]
	if !e.keeps || !k.within(e) || g.wouldCheck(e.key) || !g.looksHeld(&k.jn, e.jn, shift) {
		return false
	}
	g.passCheck()
	g.keep(k, it.entry, shift)
	d := city.Point{X: float64(shift.X), Y: float64(shift.Y)}
	held := heldItem{id: it.id, entry: len(s.entries) - 1, kind: it.kind, labels: it.labels, held: true}
	for _, h := range it.hits {
		h.rect = city.Rect{Min: h.rect.Min.Add(d), Max: h.rect.Max.Add(d)}
		g.noteHit(h.rect, h.hit)
		held.hits = append(held.hits, h)
	}
	for _, l := range it.labels {
		g.queueLabel(l)
	}
	s.log = append(s.log, held)
	s.stats.held++
	s.stats.reused++
	s.stats.keptBy[it.kind]++
	return true
}

// wouldCheck is whether the net would check the kept record under key
// were it noted now, without spending its turn.
func (g *Game) wouldCheck(key uint64) bool {
	switch {
	case g.verifyEvery > 0:
		return (g.verifyCount+1)%g.verifyEvery == 0
	case g.verifyBudget > 0:
		return g.still.stats.checked < g.verifyBudget && !g.verifyDone[key]
	}
	return false
}

// passCheck counts a held record as one the net saw and passed by, as
// checks would have.
func (g *Game) passCheck() {
	switch {
	case g.verifyEvery > 0:
		g.verifyCount++
	case g.verifyBudget > 0:
		g.verifySeen++
	}
}
