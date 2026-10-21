package render

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// bakedLook is a run of a building's steps baked into an image, where it
// goes from the pixel its foot is in, and the words painted into it.
type bakedLook struct {
	img   *ebiten.Image
	at    image.Point
	words []string
}

func (l *bakedLook) Deallocate() { l.img.Deallocate() }

// lookStats counts what the cache did this frame.
type lookStats struct {
	hits, misses, bakes int
}

// buildingFrame is everything a building's steps read this frame, worked
// out once before any of them draws.
type buildingFrame struct {
	b                  *city.Building
	selected, detailed bool
	seconds            float64
	name               string
	rise               float64
	tint               *ebiten.ColorScale
	site               city.Site
	building           bool
	dress              lot
	settle             city.Settle
	progress           float64
	settling           bool
	state              lookState
	present, live      [stepCount]bool
}

// drawBuilding draws a session's building and its work, as one item of
// the depth sort: from its baked looks where nothing in them moves.
func (g *Game) drawBuilding(screen *ebiten.Image, cam *city.Camera, b *city.Building, selected, detailed bool, seconds float64) {
	f, ok := g.buildingFrame(cam, b, selected, detailed, seconds)
	if !ok {
		g.isoBuilding(screen, cam, b, selected, detailed, seconds)
		g.buildingWork(screen, cam, b, detailed)
		return
	}
	g.buildingHits(cam, &f)
	// Its own lights are noted rather than drawn, so they are never
	// baked, but they do not move.
	for i := range f.live {
		if f.live[i] && f.present[i] && step(i) != stepLights {
			g.rec.moves()
			break
		}
	}
	for _, r := range runsOf(f.present, f.live) {
		if !r.baked || g.looks == nil || g.noLookCache {
			g.drawSteps(screen, cam, &f, r.from, r.to)
			continue
		}
		g.drawRun(screen, cam, &f, r)
	}
}

// buildingWork is what drawIso draws after a building it does not bake:
// the skip, the scaffold and the spire.
func (g *Game) buildingWork(screen *ebiten.Image, cam *city.Camera, b *city.Building, detailed bool) {
	if g.kits != nil && !b.Vacant && !g.scene.Dimmed(b) {
		g.drawSkip(screen, cam, b, nil)
	}
	if detailed && g.kits != nil && !g.viewing() && !b.Vacant && !g.scene.Dimmed(b) {
		g.drawWork(screen, cam, b)
	}
	// The spire goes over the work: a segment is half a storey, so the
	// storey being built would hide the first of them.
	if g.kits != nil && !b.Vacant && !g.scene.Dimmed(b) {
		g.drawSpire(screen, cam, b)
	}
}

// buildingFrame works out a building's frame, or says it is not drawn as
// steps: in the flat map, on a truck, or not yet built.
func (g *Game) buildingFrame(cam *city.Camera, b *city.Building, selected, detailed bool, seconds float64) (buildingFrame, bool) {
	if !detailed || g.kits == nil {
		return buildingFrame{}, false
	}
	if g.scene.Hauling(b.Session.ID) && !g.reduced {
		return buildingFrame{}, false
	}
	f := buildingFrame{b: b, selected: selected, detailed: detailed, seconds: seconds, name: buildingPiece(b)}
	f.rise = g.scene.Rising(b.Session.ID)
	f.site, f.building = g.scene.Construction(b.Session.ID)
	f.building = f.building && !b.Vacant
	if b.Vacant || f.rise <= 0 {
		return buildingFrame{}, false
	}
	var tint *ebiten.ColorScale
	switch {
	case g.scene.Dimmed(b):
		tint = dimmed()
	case g.unlit(b):
		tint = dimmed()
	case g.viewing():
		if c, ok := g.viewTint(b); ok {
			tint = viewScale(c)
		}
	case b.Blinds:
		tint = blindsTint()
	case b.Session.State == state.Unattended:
		tint = &ebiten.ColorScale{}
		tint.SetR(0.85)
		tint.SetG(0.8)
		tint.SetB(1)
	}
	fade := g.scene.HaulProgress(b.Session.ID)
	if fade < 1 {
		faded := &ebiten.ColorScale{}
		if tint != nil {
			*faded = *tint
		}
		faded.ScaleAlpha(float32(fade))
		tint = faded
	}
	if f.building {
		tint = buildTint(tint, f.site)
	}
	f.dress = g.dressing(b)
	f.settle, f.progress = g.scene.Settling(b.Session.ID)
	f.settling = f.settle != city.SettleNone && f.rise >= 1 && !g.scene.Dimmed(b) && !g.viewing()
	if f.settling && !g.reduced {
		tint = settleTint(f.settle, f.progress)
	}
	f.tint = tint

	r := g.spriteRect(cam, &f)
	dim := g.scene.Dimmed(b)
	s := lookState{
		courses:  f.rise >= 1 && courseCount(b.Session.Compactions) > 0,
		rising:   f.rise < 1,
		fading:   fade < 1,
		settling: f.settling,
		building: f.building,
		stop:     r.Area() == 0 || dim,
		dimmed:   dim,
		facade:   f.dress.Dark && f.rise >= 1 && !g.viewing(),
		beacon:   f.dress.Beacon || f.settle != city.SettleNone,
		pulse:    b.Pulse,
		sign:     f.dress.Sign,
		flags:    min(b.Flags, 3),
		selected: selected,
		spire:    spireSegments(b) > 0,
	}
	if _, pouring := g.pouring(b); pouring {
		s.pouring = true
	}
	for i := 0; i < s.flags; i++ {
		if h := g.flagHoist(b, i); h > 0 && h < 1 {
			s.hoisting = true
		}
	}
	s.celebrating = g.scene.Celebration(b.Session.ID) >= 0
	s.smoke = b.Smoke > 0
	if g.lightsOn {
		_, s.lights = g.windowLights(cam, b, f.rise, g.scene.Lights(b.Session.ID))
	}
	if !b.Packed {
		job, collecting := g.scene.CompactionJob(b.Session.ID)
		s.skip = skipPiece(shownSkip(b, job, collecting)) != ""
		s.trucks = collecting && (job.SkipTruck.Present || job.Mixer.Present)
	}
	if detailed && !g.viewing() {
		s.work = g.scene.Scaffold(b) > 0
		s.loads = len(g.scene.Loads(b.Session.ID)) > 0
	}
	f.state = s
	f.present, f.live = s.steps()
	return f, true
}

// flagHoist is how far up its pole a building's i'th flag is.
func (g *Game) flagHoist(b *city.Building, i int) float64 {
	if i < len(b.Session.PRs) {
		return g.scene.Hoist(b.Session.ID, b.Session.PRs[i].Number)
	}
	return 1
}

// spriteRect is where a building's sprite lands with a camera, risen as
// far as it has, without drawing it.
func (g *Game) spriteRect(cam *city.Camera, f *buildingFrame) city.Rect {
	atlas := g.kits.pick(cam.Zoom)
	if atlas == nil {
		return city.Rect{}
	}
	sprite, ok := atlas.Sprite(f.name, cam.Heading)
	if !ok || sprite.Page >= len(atlas.pages) {
		return city.Rect{}
	}
	foot := g.footOf(cam, f.b)
	scale := cam.Zoom / atlas.Zoom
	rise := min(1, f.rise)
	return city.RectAt(foot.X-float64(sprite.Anchor.X)*scale, foot.Y-float64(sprite.Anchor.Y)*scale*rise, float64(sprite.Rect.Dx())*scale, float64(sprite.Rect.Dy())*scale*rise)
}

// buildingHits notes what of a building answers the pointer, and queues
// its badge, every frame, whether its look is baked or not.
func (g *Game) buildingHits(cam *city.Camera, f *buildingFrame) {
	b := f.b
	r := g.spriteRect(cam, f)
	district := g.scene.City().DistrictOf(b)
	g.noteHit(r, city.Hit{Building: b, District: district})
	if f.state.stop {
		return
	}
	foot := g.footOf(cam, b)
	if f.dress.Sign && g.labelsVisible() {
		g.queueBadge(b, foot)
	}
	roofTop := r.Min.Y
	dot := math.Max(3, 6*cam.Zoom)
	for i := 0; i < f.state.flags; i++ {
		if g.flagHoist(b, i) <= 0 || i >= len(b.Session.PRs) {
			continue
		}
		x := r.Min.X + r.Width()*0.62 + float64(i)*dot*1.8
		g.noteHit(city.RectAt(x, roofTop-dot*2, dot*1.4, dot*2.5),
			city.Hit{Building: b, District: district, Flag: &city.FlagHit{Building: b, PR: b.Session.PRs[i]}})
	}
	for _, p := range smokePuffs(r, cam.Zoom, b.Smoke, f.seconds) {
		g.noteHit(city.RectAt(p.at.X-p.radius, p.at.Y-p.radius, p.radius*2, p.radius*2),
			city.Hit{Building: b, District: district, Smoke: b})
	}
}

// drawSteps draws a building's steps from one to before another,
// straight onto dst with a camera.
func (g *Game) drawSteps(dst *ebiten.Image, cam *city.Camera, f *buildingFrame, from, to step) {
	b := f.b
	r := g.spriteRect(cam, f)
	foot := g.footOf(cam, b)
	roofTop := r.Min.Y
	dot := math.Max(3, 6*cam.Zoom)
	for i := from; i < to; i++ {
		if !f.present[i] {
			continue
		}
		switch i {
		case stepCourses:
			g.drawCourses(dst, cam, b, f.tint)
		case stepSprite:
			g.kitRising(dst, cam, f.name, foot, f.tint, f.rise)
		case stepSite:
			ground := cam.WorldToScreen(b.Rect.Center())
			g.drawSite(dst, lotCorners(cam, b.Rect), ground.Y-g.kitTop(cam, f.name), f.site, cam.Zoom)
		case stepFacade:
			switch {
			case f.settling:
				g.putOut(dst, cam, b, f.name, f.settle, f.progress)
			case f.state.facade:
				g.shutter(dst, cam, b, f.name, f.dress.Boards)
			}
		case stepLights:
			g.lightBuilding(cam, b)
		case stepBeacon:
			beacon := g.theme.Color(ui.StateTone(b.Session.State))
			if b.Pulse {
				beacon = pulse(beacon, g.attention())
			}
			kerb := colorKitKerb
			if f.settle != city.SettleNone {
				fade := beaconFade(f.settle, f.progress)
				beacon.A, kerb.A = uint8(float64(beacon.A)*fade), uint8(float64(kerb.A)*fade)
			}
			radius := math.Max(2, 4*cam.Zoom)
			lamp := city.Point{X: foot.X, Y: foot.Y - radius*2}
			g.paint.Disc(dst, lamp, radius*1.6, kerb)
			g.paint.Disc(dst, lamp, radius, beacon)
		case stepSign:
			title := b.Card(g.scene.City().Time).Title
			if !g.buildingSignage(dst, cam, r, foot, title) {
				g.buildingSign(dst, r, foot, title)
			}
		case stepFlags:
			for k := 0; k < f.state.flags; k++ {
				hoist := g.flagHoist(b, k)
				if hoist <= 0 {
					continue
				}
				x := r.Min.X + r.Width()*0.62 + float64(k)*dot*1.8
				flag := g.theme.Palette.Accent
				if k < b.Merged {
					flag = g.theme.Palette.Merged
				} else if k < len(b.Session.PRs) && b.Session.PRs[k].State == claude.PRClosed {
					flag = g.theme.Palette.Resting
				}
				g.paint.Box(dst, float32(x), float32(roofTop-dot*2), float32(dot*0.4), float32(dot*2.5), colorPole)
				// The flag runs up its pole as it is hoisted (item 89).
				g.paint.Box(dst, float32(x), float32(roofTop+dot*0.5-dot*2.5*hoist), float32(dot*1.4), float32(dot), flag)
			}
		case stepCelebrate:
			if p := g.scene.Celebration(b.Session.ID); p >= 0 {
				g.celebrate(dst, city.Point{X: r.Min.X + r.Width()/2, Y: roofTop}, r.Width(), p)
			}
		case stepSmoke:
			for _, p := range smokePuffs(r, cam.Zoom, b.Smoke, f.seconds) {
				g.paint.Disc(dst, p.at, p.radius, colorSmoke)
			}
		case stepSelected:
			g.polyStroke(dst, cam, b.Rect, 2, colorSelected)
		case stepSkip:
			g.drawSkipPiece(dst, cam, b, nil)
		case stepSkipTrucks:
			g.drawSkipTrucks(dst, cam, b, nil)
		case stepWork:
			g.drawWork(dst, cam, b)
		case stepSpire:
			g.drawSpire(dst, cam, b)
		}
	}
}

// lookPad is how far past what it measures a baked look reaches, for a
// stroke's fringe and a filtered edge.
const lookPad = 4

// lookBounds is the box on screen a building's baked steps can paint in.
func (g *Game) lookBounds(cam *city.Camera, f *buildingFrame) city.Rect {
	b := f.b
	r := g.spriteRect(cam, f)
	out := r.Union(screenBounds(cam, b.Rect))
	cell := r.Width()
	dot := math.Max(3, 6*cam.Zoom)
	top := r.Min.Y - math.Max(cell*boardHeight, dot*2.5)
	right := r.Min.X + r.Width()*0.62 + 3*dot*1.8 + dot*1.4
	out = out.Union(city.Rect{Min: city.Point{X: r.Min.X, Y: top}, Max: city.Point{X: right, Y: r.Max.Y}})
	if g.signage != city.SignagePlates && g.signage != city.SignageHover {
		out = out.Union(city.RectAt(r.Min.X, top, r.Width()*3, r.Height()))
	}
	if f.present[stepCourses] {
		n := courseCount(b.Session.Compactions)
		ground := cam.WorldToScreen(b.Rect.Center())
		for i := range n {
			at := city.Point{X: ground.X, Y: ground.Y - float64(i)*courseHeight(cam.Zoom)}
			if _, _, _, rect, ok := g.kitAtPlace(cam, coursePiece(f.name, n-i), 0, at); ok {
				out = out.Union(rect)
			}
		}
		if plate, _, _, ok := g.coursePlate(cam, b); ok {
			out = out.Union(plate)
		}
	}
	if f.present[stepSkip] {
		job, collecting := g.scene.CompactionJob(b.Session.ID)
		if _, _, _, rect, ok := g.kitPlace(cam, skipPiece(shownSkip(b, job, collecting)), 0, g.skipAt(cam, b), 0); ok {
			out = out.Union(rect)
		}
	}
	if f.present[stepWork] {
		if site, ok := g.workFrame(cam, b); ok {
			lot := screenBounds(cam, b.Rect)
			lift := site.wallTop + 2*site.storey
			out = out.Union(city.Rect{
				Min: city.Point{X: lot.Min.X, Y: lot.Min.Y - lift},
				Max: city.Point{X: lot.Max.X + max(4, site.wallTop*0.09) + crateSize(site.wallTop), Y: lot.Max.Y},
			})
		}
	}
	if n := spireSegments(b); f.present[stepSpire] && n > 0 {
		s := spireOf(g.spireBase(cam, b), storeyHeight(cam.Zoom), n)
		glow := max(2, storeyHeight(cam.Zoom)*0.12) * 2.2
		out = out.Union(city.Rect{Min: city.Point{X: s.tip.X - glow, Y: s.tip.Y - glow}, Max: city.Point{X: s.tip.X + glow, Y: s.tip.Y + glow}})
	}
	return out
}

// lookCell splits where a building's foot falls into the pixel it is in
// and the phase inside that pixel, as lookPhase tells it.
func lookCell(foot city.Point) image.Point {
	cell := func(v float64) int {
		q := int(math.Round(v * lookPhaseSteps))
		return int(math.Floor(float64(q) / lookPhaseSteps))
	}
	return image.Point{X: cell(foot.X), Y: cell(foot.Y)}
}

// keyOf is the key a run of a building's steps is baked under.
func (g *Game) keyOf(cam *city.Camera, f *buildingFrame, r run) lookKey {
	b := f.b
	k := lookKey{
		id:      b.Session.ID,
		run:     r.from,
		live:    f.live,
		present: f.present,
		atlas:   g.kits.pick(cam.Zoom),
		faces:   g.faces,
		piece:   f.name,
		heading: cam.Heading,
		zoom:    cam.Zoom,
		phase:   lookPhase(g.footOf(cam, b)),
		recede:  g.viewing() && f.tint == nil,
		viewing: g.viewing(),
		reduced: g.reduced,
		signage: g.signage,
		packed:  b.Packed,
		gone:    b.Session.ConversationGone,

		compactions: b.Session.Compactions,
		litCourse:   courseLit(b.Session, g.scene.Now()),
		dark:        f.dress.Dark,
		boards:      f.dress.Boards,
		selected:    f.selected,
		spire:       spireSegments(b),
	}
	if f.tint != nil {
		k.tint, k.tinted = *f.tint, true
	}
	if f.present[stepBeacon] {
		k.beacon = g.theme.Color(ui.StateTone(b.Session.State))
	}
	if f.present[stepSign] {
		k.title = b.Card(g.scene.City().Time).Title
		k.shown = g.shareAllows(k.title)
	}
	for i := 0; i < f.state.flags; i++ {
		k.flags[i] = flagLook{hoist: g.flagHoist(b, i)}
		switch {
		case i < b.Merged:
			k.flags[i].colour = g.theme.Palette.Merged
		case i < len(b.Session.PRs) && b.Session.PRs[i].State == claude.PRClosed:
			k.flags[i].colour = g.theme.Palette.Resting
		default:
			k.flags[i].colour = g.theme.Palette.Accent
		}
	}
	if f.present[stepSkip] {
		job, collecting := g.scene.CompactionJob(b.Session.ID)
		k.skip = skipPiece(shownSkip(b, job, collecting))
	}
	if f.present[stepWork] {
		k.scaffold = g.scene.Scaffold(b)
	}
	return k
}

// drawRun draws a run of a building's steps from its baked look, baking
// it first once its key has held a frame, and straight onto the frame
// until then.
func (g *Game) drawRun(screen *ebiten.Image, cam *city.Camera, f *buildingFrame, r run) {
	key := g.keyOf(cam, f, r)
	cell := lookCell(g.footOf(cam, f.b))
	look, ok := g.looks.get(key)
	switch {
	case ok:
		g.lookStats.hits++
		if g.textSeen != nil {
			for _, w := range look.words {
				g.textSeen(w)
			}
		}
	case g.looks.steady(runID{id: f.b.Session.ID, run: r.from}, key, g.lookFrame):
		look = g.bake(cam, f, r, cell)
		if look == nil {
			g.drawSteps(screen, cam, f, r.from, r.to)
			return
		}
		g.lookStats.bakes++
		g.looks.put(key, look, look.img.Bounds().Dx()*look.img.Bounds().Dy()*4)
	default:
		g.lookStats.misses++
		g.drawSteps(screen, cam, f, r.from, r.to)
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(cell.X+look.at.X), float64(cell.Y+look.at.Y))
	g.drawing.world()
	g.recorder().Image(screen, look.img, op)
	g.punch(look.img, op.GeoM)
}

// bake draws a run of a building's steps into an image of its own, with
// the camera moved by whole pixels so the building lands inside it, and
// the frame's notes of what was drawn left as they were.
func (g *Game) bake(cam *city.Camera, f *buildingFrame, r run, cell image.Point) *bakedLook {
	box := g.lookBounds(cam, f)
	lo := image.Point{X: int(math.Floor(box.Min.X)) - lookPad, Y: int(math.Floor(box.Min.Y)) - lookPad}
	hi := image.Point{X: int(math.Ceil(box.Max.X)) + lookPad, Y: int(math.Ceil(box.Max.Y)) + lookPad}
	size := hi.Sub(lo)
	if size.X <= 0 || size.Y <= 0 {
		return nil
	}
	look := &bakedLook{img: ebiten.NewImage(size.X, size.Y), at: lo.Sub(cell)}
	moved := *cam
	moved.Offset = cam.Offset.Sub(city.Point{X: float64(lo.X), Y: float64(lo.Y)}.Scale(1 / cam.Zoom))
	seen, drawing := g.textSeen, g.drawing
	g.textSeen = func(s string) {
		look.words = append(look.words, s)
		if seen != nil {
			seen(s)
		}
	}
	uncut := g.paintUncut
	g.baking, g.paintUncut = true, true
	g.drawSteps(look.img, &moved, f, r.from, r.to)
	g.baking, g.paintUncut = false, uncut
	g.textSeen, g.drawing = seen, drawing
	return look
}

// runID is one run of one building, for telling whether its key held.
type runID struct {
	id  string
	run step
}

// lookForget is how many frames a building may go undrawn before what
// was seen of it is dropped.
const lookForget = 600

// nextLookFrame starts a frame of the looks: counted, its counts reset.
func (g *Game) nextLookFrame() {
	if g.looks == nil {
		g.looks = newLookCache[lookKey, *bakedLook](lookCacheCap)
	}
	g.lookFrame++
	g.lookStats = lookStats{}
	if g.lookFrame%lookForget == 0 {
		g.looks.forget(g.lookFrame - lookForget)
	}
}
