package render

import (
	"fmt"
	"hash/maphash"
	"image"
	"math"
	"strconv"
	"strings"
	"unsafe"

	"botropolis.city/botropolis/pkg/city"
)

// standingKey is what every still drawable reads alike, and each still
// thing's values that change without a snapshot, folded once a frame:
// while it holds from one frame to the next, nothing still on the map
// would be drawn otherwise (item 212). TestStillKeyReads holds every
// still drawable's key and draw to what this reads.
// Where why is not nil, each input is appended to it by name.
func (g *Game) standingKey(selected *city.Building, why *[]standingInput) uint64 {
	s := g.scene
	if !g.frameBase.ok {
		g.frameBase = g.frameBaseKey()
	}
	c := s.City()
	now := s.Now()
	f := standingFold{why: why}
	f.put("base", g.frameBase.all)
	f.put("house", g.frameBase.house)
	f.put("house-wide", g.houseWide)
	f.put("generation", uint64(s.Generation()))
	f.put("ground", uint64(s.GroundGeneration()))
	f.put("view", uint64(s.View()))
	f.put("night", boolBits(c.Night))
	f.put("labels", boolBits(g.labelsVisible()))
	f.put("scenery", boolBits(s.Scenery()))
	f.put("detailed", boolBits(s.Detailed()))
	f.put("plant lights", math.Float64bits(s.PlantLights()))
	f.put("selected", uint64(uintptr(unsafe.Pointer(selected))))
	// The clinic's plate counts what needs you, which a snooze running
	// out changes with nothing else (item 280).
	f.put("clinics", uint64(s.ClinicCount()))
	f.put("focused", maphash.String(inkSeed, s.FocusedGroup()))
	// Moved by part of a pixel, nothing drawn lands where it did.
	if g.rec != nil {
		o := g.rec.origin
		frac := func(v float64) uint64 { return uint64(math.Round((v-math.Floor(v))*64)) % 64 }
		f.put("origin", keyMix(frac(o.X), frac(o.Y)))
	}
	var stirring uint64
	for id := range g.stirring {
		stirring ^= maphash.String(inkSeed, id)
	}
	f.put("stirring", stirring)
	viewing := g.viewing()
	for _, b := range c.Buildings() {
		id := b.Session.ID
		f.putFor(id, "dimmed", boolBits(s.Dimmed(b)))
		f.putFor(id, "unlit", boolBits(g.unlit(b)))
		f.putFor(id, "backing off", boolBits(g.backingOff(id)))
		f.putFor(id, "scaffold", math.Float64bits(s.Scaffold(b)))
		f.putFor(id, "course", boolBits(courseLit(&b.Session, now)))
		f.putFor(id, "rising", math.Float64bits(s.Rising(id)))
		f.putFor(id, "lights", math.Float64bits(s.Lights(id)))
		f.putFor(id, "mowing", math.Float64bits(s.Mowing(id)))
		if site, ok := s.Construction(id); ok {
			f.putFor(id, "site", keyMix(math.Float64bits(site.Fence), math.Float64bits(site.Scaffold), math.Float64bits(site.Crane),
				math.Float64bits(site.Floors), math.Float64bits(site.Lights), math.Float64bits(site.Opacity)))
		}
		if viewing {
			if tint, ok := g.viewTint(b); ok {
				f.putFor(id, "tint", uint64(tint.R)<<24|uint64(tint.G)<<16|uint64(tint.B)<<8|uint64(tint.A))
			}
		}
		for i := range min(b.Flags, 3) {
			f.putFor(id, hoistNames[i], math.Float64bits(g.flagHoist(b, i)))
		}
	}
	return f.key
}

// whyHoldDefault is whether a new game keeps its standing key input by
// input: a debug build's does, for the frame log (item 212).
var whyHoldDefault = false

// hoistNames name a building's three flags among the standing key's
// inputs.
var hoistNames = [3]string{"hoist 0", "hoist 1", "hoist 2"}

// standingInput is one input of the standing key: the building it is
// of, if any, its name, and its value.
type standingInput struct {
	id, name string
	v        uint64
}

// standingFold folds the standing key's inputs in turn, and keeps them
// by name where why is set.
type standingFold struct {
	key uint64
	why *[]standingInput
}

func (f *standingFold) put(name string, v uint64) { f.putFor("", name, v) }

func (f *standingFold) putFor(id, name string, v uint64) {
	f.key = keyMix(f.key, v)
	if f.why != nil {
		*f.why = append(*f.why, standingInput{id: id, name: name, v: v})
	}
}

// holdBroke is the first of the standing key's inputs that changed
// since the last frame, named as the frame log names it, or nothing
// when the key held or its inputs were not kept. A building come or
// gone is "buildings".
func (g *Game) holdBroke() string {
	s := &g.still
	if s.standing == s.lastStanding || len(s.why) == 0 || len(s.lastWhy) == 0 {
		return ""
	}
	for i, in := range s.why {
		if i >= len(s.lastWhy) {
			return "buildings"
		}
		was := s.lastWhy[i]
		if was.id != in.id || was.name != in.name {
			return "buildings"
		}
		if was.v == in.v {
			continue
		}
		if in.id == "" {
			return in.name
		}
		return in.id + " " + in.name
	}
	return "buildings"
}

// holdLine is what a frame held and ran, by kind where it recorded
// afresh, and what broke its hold, as one line of the frame log.
func holdLine(st stillStats, broke string) string {
	var fresh []string
	for k, n := range st.freshBy {
		if n > 0 {
			fresh = append(fresh, kindTags[k]+":"+strconv.Itoa(n))
		}
	}
	if broke == "" {
		broke = "nothing"
	}
	return fmt.Sprintf("hold: held=%d ran=%d ran-kept=%d aged-held=%d fresh=%s broke=%s",
		st.held, st.ran, st.ranKept, st.agedHeld, strings.Join(fresh, ","), broke)
}

// kindTags name the kinds in the frame log.
var kindTags = [kindsKept]string{"", "district-floor", "lot-floor", "house", "monument", "sign", "tree", "bridge",
	"fog", "landmark", "stack", "tower", "lines"}

// heldItem is a still thing as a frame noted it: who it is, the entry its
// record became (-1 when it was not one to keep), its kind, and the hits
// and labels it noted, for a frame whose standing key holds to note
// again without running its draw (item 212).
type heldItem struct {
	at     itemAt
	entry  int
	kind   recordKind
	hits   []spriteHit
	labels []pendingLabel
	// held is whether this frame noted it as the last did.
	held bool
}

// itemAt is where a keyed item stands: its footprint and how high. Two
// things on one footprint, the plant and its stack, come in the order
// drawIso appends them, which the sort keeps.
type itemAt struct {
	turned city.Rect
	level  int
}

// nextLog is the log's next item, empty, keeping the room its hits and
// labels had two frames ago.
func (s *stillFrame) nextLog() *heldItem {
	n := len(s.log)
	if n < cap(s.log) {
		s.log = s.log[:n+1]
	} else {
		s.log = append(s.log, heldItem{})
	}
	it := &s.log[n]
	*it = heldItem{hits: it.hits[:0], labels: it.labels[:0]}
	return it
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
	at := itemAt{turned: d.turned, level: d.level}
	if *hold {
		if n >= len(s.lastLog) || s.lastLog[n].at != at {
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
	it := s.nextLog()
	it.at, it.entry, it.kind = at, -1, s.notedKind
	if len(s.entries) == entries+1 && s.entries[entries].keeps && s.notedKind < kindsKept {
		it.entry = entries
		s.stats.ranKept++
	}
	g.mu.Lock()
	it.hits = append(it.hits, g.frameHits[hits:]...)
	g.mu.Unlock()
	it.labels = append(it.labels, g.mapLabels[labels:]...)
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
	held := s.nextLog()
	held.at, held.entry, held.kind, held.held = it.at, len(s.entries)-1, it.kind, true
	for _, h := range it.hits {
		h.rect = city.Rect{Min: h.rect.Min.Add(d), Max: h.rect.Max.Add(d)}
		g.noteHit(h.rect, h.hit)
		held.hits = append(held.hits, h)
	}
	// Copied, not shared: each log keeps its items' room for reuse.
	held.labels = append(held.labels, it.labels...)
	for _, l := range it.labels {
		g.queueLabel(l)
	}
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

// dropActive is the frame's entries without those that show activity, in
// place: the frame the frame test compares two cities by, with the same
// activity, none (item 193); with the log pointed at where each entry it names
// went, or at none, so the next frame holds nothing in another's place.
func (s *stillFrame) dropActive() []entry {
	at := make([]int, len(s.entries))
	out := s.entries[:0]
	for i, e := range s.entries {
		at[i] = -1
		if !e.active {
			at[i] = len(out)
			out = append(out, e)
		}
	}
	for i := range s.log {
		if e := s.log[i].entry; e >= 0 && e < len(at) {
			s.log[i].entry = at[e]
		}
	}
	for i := range s.agedLog {
		if e := s.agedLog[i].entry; e >= 0 && e < len(at) {
			s.agedLog[i].entry = at[e]
		}
	}
	return out
}

// agedItem is an aged lot's drawable as a frame noted it: its
// district's root, whether it is the district's floor or its lots,
// the entry its record became (-1 when it was not one to keep), and
// whether it drew nothing at all (item 212).
type agedItem struct {
	root  string
	floor bool
	entry int
	none  bool
	held  bool
}

// holdingAged is whether this frame notes the aged lots as the last did:
// as holding is for the city's still things, but before the items are
// made, so without their fade.
func (g *Game) holdingAged() (image.Point, bool) {
	s := &g.still
	if !s.agedLogged || s.ground != s.lastGround || g.noKeyed || g.noStill || g.noReuse {
		return image.Point{}, false
	}
	return pixelShift(s.kept.origin, g.rec.origin)
}

// noteAged notes aged item n, root's floor or lots, as the last frame
// noted it while hold lasts and it arrives where the last frame had it,
// otherwise by running it.
func (g *Game) noteAged(n int, root string, floor bool, hold *bool, shift image.Point, run func()) {
	s := &g.still
	if *hold {
		if n >= len(s.lastAgedLog) || s.lastAgedLog[n].root != root || s.lastAgedLog[n].floor != floor {
			*hold = false
			s.stats.orderBroke++
		} else if g.holdAged(&s.lastAgedLog[n], shift) {
			return
		}
	}
	entries := len(s.entries)
	run()
	it := agedItem{root: root, floor: floor, entry: -1, none: len(s.entries) == entries}
	if len(s.entries) == entries+1 && s.entries[entries].keeps {
		it.entry = entries
	}
	s.agedLog = append(s.agedLog, it)
	s.stats.agedRan++
}

// holdAged notes an aged lot as the last frame did, moved by shift: its
// kept record, or nothing where it drew nothing, which only a snapshot
// changes. It does not when its record was not kept, when the net checks
// it this frame, or when its looks have gone.
func (g *Game) holdAged(it *agedItem, shift image.Point) bool {
	s := &g.still
	held := *it
	held.held = true
	if it.none {
		s.agedLog = append(s.agedLog, held)
		s.stats.agedHeld++
		return true
	}
	k := &s.kept
	if it.entry < 0 || it.entry >= len(k.entries) {
		return false
	}
	e := k.entries[it.entry]
	if !e.keeps || !k.within(e) || g.wouldCheck(e.key) || !g.looksHeld(&k.jn, e.jn, shift) {
		return false
	}
	g.passCheck()
	g.keep(k, it.entry, shift)
	held.entry = len(s.entries) - 1
	s.agedLog = append(s.agedLog, held)
	s.stats.agedHeld++
	s.stats.reused++
	kind := kindLotFloor
	if it.floor {
		kind = kindDistrictFloor
	}
	s.stats.keptBy[kind]++
	return true
}

// groundKey is what the aged lots are drawn from, folded once a frame as
// the standing key is, but of only what they read: the frame's base,
// the house-wide key, the scene's generations and view, night, where the
// camera falls within a pixel, and each building's dimming and mowing.
// It leaves out what only buildings read, the stirring sessions among it,
// so a tool call's load does not stop the ground holding (item 212).
// TestStillKeyReads holds the aged lots' keys and draws to what it reads.
func (g *Game) groundKey() uint64 {
	s := g.scene
	if !g.frameBase.ok {
		g.frameBase = g.frameBaseKey()
	}
	c := s.City()
	key := keyMix(g.frameBase.all, g.frameBase.house, g.houseWide,
		uint64(s.Generation()), uint64(s.GroundGeneration()), uint64(s.View()), boolBits(c.Night))
	if g.rec != nil {
		o := g.rec.origin
		frac := func(v float64) uint64 { return uint64(math.Round((v-math.Floor(v))*64)) % 64 }
		key = keyMix(key, frac(o.X), frac(o.Y))
	}
	for _, b := range c.Buildings() {
		key = keyMix(key, boolBits(s.Dimmed(b)), math.Float64bits(s.Mowing(b.Session.ID)))
	}
	return key
}
