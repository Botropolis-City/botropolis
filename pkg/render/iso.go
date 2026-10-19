package render

import (
	"image/color"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/colorm"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// poly fills a world rectangle as its projected outline.
func (g *Game) poly(screen *ebiten.Image, cam *city.Camera, r city.Rect, c color.NRGBA) {
	var path vector.Path
	for i, corner := range cam.Corners(r) {
		p := corner.Add(cam.Offset).Scale(cam.Zoom)
		if i == 0 {
			path.MoveTo(float32(p.X), float32(p.Y))
		} else {
			path.LineTo(float32(p.X), float32(p.Y))
		}
	}
	path.Close()
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(c)
	vector.FillPath(screen, &path, nil, op)
}

func (g *Game) polyStroke(screen *ebiten.Image, cam *city.Camera, r city.Rect, width float32, c color.NRGBA) {
	var path vector.Path
	for i, corner := range cam.Corners(r) {
		p := corner.Add(cam.Offset).Scale(cam.Zoom)
		if i == 0 {
			path.MoveTo(float32(p.X), float32(p.Y))
		} else {
			path.LineTo(float32(p.X), float32(p.Y))
		}
	}
	path.Close()
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(c)
	vector.StrokePath(screen, &path, &vector.StrokeOptions{Width: width}, op)
}

// drawSprite draws a sheet sprite with its top-left at a screen point,
// scaled uniformly.
func (g *Game) drawSprite(screen *ebiten.Image, img *ebiten.Image, at city.Point, scale float64, tint *ebiten.ColorScale) {
	if img == nil {
		return
	}
	g.drawing.world()
	// A view drains the colour out of everything it has nothing to say
	// about, and draining colour is a mixing of channels that a scale
	// cannot do — so those sprites go through a colour matrix instead.
	if g.viewing() && tint == nil {
		op := &colorm.DrawImageOptions{}
		op.GeoM.Scale(scale, scale)
		op.GeoM.Translate(at.X, at.Y)
		op.Filter = ebiten.FilterLinear
		colorm.DrawImage(screen, img, recedeMatrix(), op)
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(at.X, at.Y)
	if tint != nil {
		op.ColorScale = *tint
	}
	op.Filter = ebiten.FilterLinear
	screen.DrawImage(img, op)
}

// footprint is the projected diamond of a world rect: its top corner and
// its width on screen.
func footprint(cam *city.Camera, r city.Rect) (top city.Point, width float64) {
	c := cam.Corners(r)
	top = cam.WorldToScreen(r.Min)
	minX, maxX := c[0].X, c[0].X
	for _, p := range c {
		minX, maxX = math.Min(minX, p.X), math.Max(maxX, p.X)
		if sp := cam.WorldToScreen(cam.Unproject(p)); sp.Y < top.Y {
			top = sp
		}
	}
	width = (maxX - minX) * cam.Zoom
	return top, width
}

// isoGround tiles the visible world with flat grass, one Kenney tile per
// building cell.
// isoGround tiles the plan's ground: grass inside the map's edge, the
// river down the east side, and a tree on every cell the plan planted;
// beyond the edge there is nothing to see.
func (g *Game) isoGround(screen *ebiten.Image, cam *city.Camera, c *city.City, width, height float64) {
	screen.Fill(colorVoid)
	bounds := c.Bounds()
	if bounds.Area() == 0 {
		return
	}
	// The island's texture paints the plan's grass and the river too, so
	// the river's banks can wander and its mouths blend. Item 85.
	g.shore(screen, cam, c)
	g.isoRails(screen, cam, c)
}

// isoTree plants one of the plan's trees, back to front with everything else.
func (g *Game) isoTree(screen *ebiten.Image, cam *city.Camera, t city.Tree, tint *ebiten.ColorScale) {
	g.kit(screen, cam, treePiece(t), 0, t.At, tint)
}

// isoPlaza is the civic centre's floor and its fountain.
func (g *Game) isoPlaza(screen *ebiten.Image, cam *city.Camera, c *city.City) {
	if c.Plaza.Area() == 0 {
		return
	}
	g.poly(screen, cam, c.Plaza, colorKitConcrete)
	g.polyStroke(screen, cam, c.Plaza, 2, colorKitKerb)
}

// SprayPeriod is how long one of the fountain's spray frames holds. It
// is slow on purpose: the fountain means nothing, so it should not
// catch the eye the way a beacon does.
const SprayPeriod = 0.9

// sprayFrame is which frame of a cycle is showing at a moment. With
// motion reduced the animation clock stands still, so the fountain
// does too.
func sprayFrame(seconds float64, frames int) int {
	if frames <= 0 {
		return 0
	}
	n := int(math.Floor(seconds/SprayPeriod)) % frames
	if n < 0 {
		n += frames
	}
	return n
}

// fountain is the plaza's centrepiece: a stone basin with a lip, a
// column and a disc of water in the plant's steel blue, modelled in
// the pipeline because no kit on disk has a fountain.
func (g *Game) fountain(screen *ebiten.Image, cam *city.Camera, c *city.City, seconds float64) {
	if c.Fountain.Area() == 0 {
		return
	}
	centre := c.Fountain.Center()
	if g.kits == nil {
		g.circle(screen, cam, centre, city.Tile*1.1, colorKerb)
		g.circle(screen, cam, centre, city.Tile, colorWater)
		g.circle(screen, cam, centre, city.Tile*0.35, colorWaterLight)
		return
	}
	g.kit(screen, cam, kitFountain[sprayFrame(seconds, len(kitFountain))], 0, centre, nil)
}

// lamp is a post at an avenue crossing; at night its head glows.
func (g *Game) lamp(screen *ebiten.Image, cam *city.Camera, cell city.Cell, night bool) {
	r := g.kit(screen, cam, kitLamp, 0, cell.Center(), nil)
	if night && r.Area() > 0 {
		glow(screen, city.Point{X: r.Min.X + r.Width()*0.5, Y: r.Min.Y + r.Height()*0.12}, lit(7*cam.Zoom, minLampGlow), boost(colorLampGlow, cam.Zoom))
	}
}

func (g *Game) isoDistrict(screen *ebiten.Image, cam *city.Camera, d *city.District, hovered bool) {
	fill, kerb := colorKitFloor, colorKitKerb
	if hovered {
		fill = colorKitFloorHi
	}
	if g.lit.FadesDistrict(d) {
		fill, kerb = recededGround(fill), recededGround(kerb)
	}
	g.poly(screen, cam, d.Rect, fill)
	g.polyStroke(screen, cam, d.Rect, 2, kerb)
}

const (
	poleSpacing = 1.5 * city.BuildingSize // world units between poles
	poleHeight  = 34.0                    // screen px at zoom 1
	wireSag     = 10.0
	sparkPeriod = 2.5 // seconds for a spark to travel a line at a middling rate
)

var (
	colorPowerPole = color.NRGBA{0x3a, 0x30, 0x26, 0xff}
)

// powerLines draws each line as poles with sagging wires, warm and thick
// for fresh tokens, cool and thin for cache reads, with sparks running
// from the plant to the building at a pace set by the token rate.
func (g *Game) powerLines(screen *ebiten.Image, c *city.City, cam *city.Camera, hover city.Hit, seconds float64) {
	if !g.shows(city.NetworkWires) {
		return
	}
	lines := c.PowerLines()
	for _, line := range lines {
		from, to := line.From, line.To
		length := math.Hypot(to.X-from.X, to.Y-from.Y)
		poles := max(2, int(length/poleSpacing)+1)
		h := poleHeight * cam.Zoom
		var tops []city.Point
		for i := 0; i < poles; i++ {
			t := float64(i) / float64(poles-1)
			foot := cam.WorldToScreen(city.Point{X: from.X + (to.X-from.X)*t, Y: from.Y + (to.Y-from.Y)*t})
			if i > 0 && i < poles-1 {
				vector.StrokeLine(screen, float32(foot.X), float32(foot.Y), float32(foot.X), float32(foot.Y-h), float32(math.Max(1, 2*cam.Zoom)), colorPowerPole, false)
				vector.StrokeLine(screen, float32(foot.X-4*cam.Zoom), float32(foot.Y-h+3*cam.Zoom), float32(foot.X+4*cam.Zoom), float32(foot.Y-h+3*cam.Zoom), float32(math.Max(1, 1.5*cam.Zoom)), colorPowerPole, false)
			}
			tops = append(tops, city.Point{X: foot.X, Y: foot.Y - h})
		}
		highlight := hover.Line != nil && hover.Line.Building == line.Building
		for i := 1; i < len(tops); i++ {
			a, b := tops[i-1], tops[i]
			mid := city.Point{X: (a.X + b.X) / 2, Y: (a.Y+b.Y)/2 + wireSag*cam.Zoom}
			var cached, fresh vector.Path
			cached.MoveTo(float32(a.X), float32(a.Y+2*cam.Zoom))
			cached.QuadTo(float32(mid.X), float32(mid.Y+2*cam.Zoom), float32(b.X), float32(b.Y+2*cam.Zoom))
			fresh.MoveTo(float32(a.X), float32(a.Y))
			fresh.QuadTo(float32(mid.X), float32(mid.Y), float32(b.X), float32(b.Y))
			strokeWire := func(path *vector.Path, width float32, col color.NRGBA) {
				op := &vector.DrawPathOptions{AntiAlias: true}
				op.ColorScale.ScaleWithColor(col)
				vector.StrokePath(screen, path, &vector.StrokeOptions{Width: width}, op)
			}
			cachedCol, freshCol := colorLineCached, colorLineFresh
			// Spend paints the carrier, not only what it runs to: a wire
			// by the rate it is carrying. Health draws the same wires
			// plain, because there the signal is the one that is missing.
			if t := g.scene.WireTint(line, lines); t.Known {
				cachedCol, freshCol = ui.Sequential.At(t.Value), ui.Sequential.At(t.Value)
			}
			strokeWire(&cached, lineWidth(line.Cached, 1, 3), cachedCol)
			strokeWire(&fresh, lineWidth(line.Fresh, 1, 4), freshCol)
			if highlight {
				strokeWire(&fresh, 2, colorHighlight)
			}
		}
		// Sparks: one per line at a middling rate, more and faster as it climbs.
		rate := line.Fresh + line.Cached/20
		if rate <= 0 {
			continue
		}
		speed := math.Min(3, math.Max(0.3, math.Log10(rate/1000+1)))
		count := 1 + min(2, int(speed))
		for k := 0; k < count; k++ {
			t := math.Mod(seconds*speed/sparkPeriod+float64(k)/float64(count), 1)
			p := wirePoint(tops, t, wireSag*cam.Zoom)
			vector.FillCircle(screen, float32(p.X), float32(p.Y), float32(math.Max(1.5, 2.5*cam.Zoom)), colorPlantCore, true)
		}
	}
}

// wirePoint is the point a fraction t along the sagging wire.
func wirePoint(tops []city.Point, t, sag float64) city.Point {
	if len(tops) < 2 {
		return city.Point{}
	}
	span := t * float64(len(tops)-1)
	i := min(len(tops)-2, int(span))
	u := span - float64(i)
	a, b := tops[i], tops[i+1]
	mid := city.Point{X: (a.X + b.X) / 2, Y: (a.Y+b.Y)/2 + sag}
	// Quadratic Bezier through the sag point.
	x := (1-u)*(1-u)*a.X + 2*(1-u)*u*mid.X + u*u*b.X
	y := (1-u)*(1-u)*a.Y + 2*(1-u)*u*mid.Y + u*u*b.Y
	return city.Point{X: x, Y: y}
}

var colorNightOverlay = color.NRGBA{0x08, 0x0c, 0x24, 0x70}

// isoBuilding draws a session as a stacked building on its footprint, or a
// flat diamond in the map view.
func (g *Game) isoBuilding(screen *ebiten.Image, cam *city.Camera, b *city.Building, selected, detailed bool, seconds float64) {
	if !detailed || g.kits == nil {
		g.poly(screen, cam, b.Rect, blockColor(b, g.attention()))
		g.polyStroke(screen, cam, b.Rect, 1, colorKerb)
		if selected {
			g.polyStroke(screen, cam, b.Rect, 2, colorSelected)
		}
		return
	}
	rise := g.scene.Rising(b.Session.ID)
	// A session on a truck is drawn on the truck, not where it is going;
	// under reduced motion it fades in there instead (item 86).
	if g.scene.Hauling(b.Session.ID) && !g.reduced {
		return
	}
	site, building := g.scene.Construction(b.Session.ID)
	building = building && !b.Vacant
	drawSite := func() {
		if building {
			foot := cam.WorldToScreen(b.Rect.Center())
			g.drawSite(screen, lotCorners(cam, b.Rect), foot.Y-g.kitTop(cam, buildingPiece(b)), site, cam.Zoom)
		}
	}
	if b.Vacant || rise <= 0 {
		g.vacantPlot(screen, cam, b, selected)
		drawSite()
		return
	}
	var tint *ebiten.ColorScale
	switch {
	case g.scene.Dimmed(b):
		tint = dimmed()
	case g.unlit(b):
		tint = dimmed()
	case g.viewing():
		// In a view the city recedes and only what the view has
		// something to say about is painted in its colour. Leaving the
		// tint nil is deliberate: that is what drains a sprite, which a
		// scale cannot do because draining mixes channels.
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
	if fade := g.scene.HaulProgress(b.Session.ID); fade < 1 {
		faded := &ebiten.ColorScale{}
		if tint != nil {
			*faded = *tint
		}
		faded.ScaleAlpha(float32(fade))
		tint = faded
	}
	if building {
		tint = buildTint(tint, site)
	}
	dress := g.dressing(b)
	settle, progress := g.scene.Settling(b.Session.ID)
	settling := settle != city.SettleNone && rise >= 1 && !g.scene.Dimmed(b) && !g.viewing()
	if settling && !g.reduced {
		tint = settleTint(settle, progress)
	}
	if rise >= 1 {
		g.drawCourses(screen, cam, b, tint)
	}
	r := g.kitRising(screen, cam, buildingPiece(b), g.footOf(cam, b), tint, rise)
	drawSite()
	district := g.scene.City().DistrictOf(b)
	g.noteHit(r, city.Hit{Building: b, District: district})
	if r.Area() == 0 || g.scene.Dimmed(b) {
		return
	}
	switch {
	case settling:
		g.putOut(screen, cam, b, buildingPiece(b), settle, progress)
	case dress.Dark && rise >= 1 && !g.viewing():
		g.shutter(screen, cam, b, buildingPiece(b), dress.Boards)
	}
	// The state light: a beacon over the door in the state's own colour,
	// pulsing for needs-you, so the palette reads the same as the strip.
	foot := g.footOf(cam, b)
	if dress.Beacon || settle != city.SettleNone {
		beacon := g.theme.Color(ui.StateTone(b.Session.State))
		if b.Pulse {
			beacon = pulse(beacon, g.attention())
		}
		kerb := colorKitKerb
		if settle != city.SettleNone {
			fade := beaconFade(settle, progress)
			beacon.A, kerb.A = uint8(float64(beacon.A)*fade), uint8(float64(kerb.A)*fade)
		}
		radius := math.Max(2, 4*cam.Zoom)
		vector.FillCircle(screen, float32(foot.X), float32(foot.Y-radius*2), float32(radius*1.6), kerb, true)
		vector.FillCircle(screen, float32(foot.X), float32(foot.Y-radius*2), float32(radius), beacon, true)
	}
	roofTop := r.Min.Y
	dot := math.Max(3, 6*cam.Zoom)
	// Signage is paint on the building, not chrome: like a tower's name
	// it stays when h hides the interface.
	if dress.Sign {
		title := b.Card(g.scene.City().Time).Title
		if !g.buildingSignage(screen, cam, r, foot, title) {
			g.buildingSign(screen, r, foot, title)
		}
		if g.labelsVisible() {
			g.queueBadge(b, foot)
		}
	}
	// One flag per PR, coloured by its state: open in the accent, merged
	// green, closed slate.
	for i := 0; i < min(b.Flags, 3); i++ {
		hoist := 1.0
		if i < len(b.Session.PRs) {
			hoist = g.scene.Hoist(b.Session.ID, b.Session.PRs[i].Number)
		}
		if hoist <= 0 {
			continue
		}
		x := r.Min.X + r.Width()*0.62 + float64(i)*dot*1.8
		flag := g.theme.Palette.Accent
		if i < b.Merged {
			flag = g.theme.Palette.Merged
		} else if i < len(b.Session.PRs) && b.Session.PRs[i].State == claude.PRClosed {
			flag = g.theme.Palette.Resting
		}
		vector.FillRect(screen, float32(x), float32(roofTop-dot*2), float32(dot*0.4), float32(dot*2.5), colorPole, false)
		// The flag runs up its pole as it is hoisted (item 89).
		vector.FillRect(screen, float32(x), float32(roofTop+dot*0.5-dot*2.5*hoist), float32(dot*1.4), float32(dot), flag, false)
		if i < len(b.Session.PRs) {
			g.noteHit(city.RectAt(x, roofTop-dot*2, dot*1.4, dot*2.5),
				city.Hit{Building: b, District: g.scene.City().DistrictOf(b), Flag: &city.FlagHit{Building: b, PR: b.Session.PRs[i]}})
		}
	}
	if p := g.scene.Celebration(b.Session.ID); p >= 0 {
		g.celebrate(screen, city.Point{X: r.Min.X + r.Width()/2, Y: roofTop}, r.Width(), p)
	}
	for _, p := range smokePuffs(r, cam.Zoom, b.Smoke, seconds) {
		vector.FillCircle(screen, float32(p.at.X), float32(p.at.Y), float32(p.radius), colorSmoke, true)
		g.noteHit(city.RectAt(p.at.X-p.radius, p.at.Y-p.radius, p.radius*2, p.radius*2),
			city.Hit{Building: b, District: g.scene.City().DistrictOf(b), Smoke: b})
	}
	if selected {
		g.polyStroke(screen, cam, b.Rect, 2, colorSelected)
	}
}

// puff is one plume of smoke on screen.
type puff struct {
	at     city.Point
	radius float64
}

// smokePuffs is the smoke rising off a roof drawn at r, one plume per
// API error, up to three.
func smokePuffs(r city.Rect, zoom float64, count int, seconds float64) []puff {
	dot := math.Max(3, 6*zoom)
	var out []puff
	for i := 0; i < min(count, 3); i++ {
		phase := math.Mod(seconds*0.4+float64(i)*0.33, 1)
		at := city.Point{X: r.Min.X + r.Width()*0.5 + float64(i)*dot*1.5 + 3*zoom*math.Sin(phase*6), Y: r.Min.Y - phase*18*zoom}
		out = append(out, puff{at: at, radius: (3 + phase*3) * zoom})
	}
	return out
}

// isoLandmark draws a kit piece standing on a world rect's centre and
// notes where it landed for the pointer.
func (g *Game) isoLandmark(screen *ebiten.Image, cam *city.Camera, r city.Rect, name string, tint *ebiten.ColorScale, hit city.Hit) {
	if g.kits == nil {
		g.poly(screen, cam, r, colorPlant)
		return
	}
	g.noteHit(g.kit(screen, cam, name, landmarkTurn, r.Center(), tint), hit)
}

// noteHit remembers a drawn sprite for the pointer; drawIso gathers
// them front-last.
// Where the water tower's tank face and leg column sit in its sprite,
// as fractions of the sprite's box, for the name painted on it.
const (
	towerFaceLeft, towerFaceRight = 0.28, 0.72
	towerFaceTop, towerFaceBottom = 0.23, 0.38
	towerLegLeft, towerLegRight   = 0.42, 0.58
	towerLegTop, towerLegBottom   = 0.48, 0.85
)

// towerSign paints an MCP server's name on its tower: across the tank
// when that reads, up the legs when the name is long, and not at all
// when the tower is too small on screen — then the hover plate names it.
func (g *Game) towerSign(screen *ebiten.Image, r city.Rect, name string) {
	if r.Area() == 0 || name == "" {
		return
	}
	span := func(l, rt, t, b float64) city.Rect {
		return city.Rect{
			Min: city.Point{X: r.Min.X + r.Width()*l, Y: r.Min.Y + r.Height()*t},
			Max: city.Point{X: r.Min.X + r.Width()*rt, Y: r.Min.Y + r.Height()*b},
		}
	}
	sign, ok := ui.LayoutSign(name, span(towerFaceLeft, towerFaceRight, towerFaceTop, towerFaceBottom),
		span(towerLegLeft, towerLegRight, towerLegTop, towerLegBottom), g.faces.Measure)
	if !ok {
		return
	}
	colour := colorKitKerb
	if sign.Vertical {
		colour = colorText
	}
	g.sign(screen, sign, colour)
}

// Where a session's name goes on its building, as fractions of a tile
// on screen: a fascia band over the door on the front face, a strip up
// the flank for a tall one, and the room above the roofline a
// billboard may stand in.
// A building sprite is one cell wide whatever its height, so the sign's
// bands are measured against the sprite's width and not its height or
// the zoom: a fascia is a storey, not a fraction of a tower.
const (
	fasciaWidth  = 0.40
	fasciaHeight = 0.13
	fasciaAbove  = 0.08
	flankWidth   = 0.17
	flankInset   = 0.74
	flankFoot    = 0.22
	flankHead    = 0.06
	boardWidth   = 0.95
	boardHeight  = 0.42
)

// buildingSign paints a session's title on its building the way a firm
// puts its name on an office: over the door when it is short, up the
// side of a tall one, and on a rooftop billboard when it is too long
// for either. Nothing under seven pixels — the hover plate has the
// name then, as it does for a tower.
func (g *Game) buildingSign(screen *ebiten.Image, r city.Rect, foot city.Point, name string) {
	if r.Area() == 0 || name == "" {
		return
	}
	cell := r.Width()
	centre := r.Min.X + cell/2
	fascia := city.RectAt(foot.X-cell*fasciaWidth/2, foot.Y-cell*(fasciaAbove+fasciaHeight), cell*fasciaWidth, cell*fasciaHeight)
	flankTop := r.Min.Y + cell*flankHead
	flank := city.RectAt(r.Min.X+cell*flankInset-cell*flankWidth/2, flankTop, cell*flankWidth, math.Max(0, foot.Y-cell*flankFoot-flankTop))
	board := city.RectAt(centre-cell*boardWidth/2, r.Min.Y-cell*boardHeight, cell*boardWidth, cell*boardHeight)
	sign, ok := ui.LayoutBuildingSign(name, fascia, flank, board, g.faces.Measure)
	if !ok {
		return
	}
	if sign.Billboard() {
		for _, post := range sign.Posts {
			vector.FillRect(screen, float32(post.Min.X), float32(post.Min.Y), float32(post.Width()), float32(post.Height()), colorKitKerb, false)
		}
		vector.FillRect(screen, float32(sign.Panel.Min.X), float32(sign.Panel.Min.Y), float32(sign.Panel.Width()), float32(sign.Panel.Height()), colorKitFloor, false)
		vector.StrokeRect(screen, float32(sign.Panel.Min.X), float32(sign.Panel.Min.Y), float32(sign.Panel.Width()), float32(sign.Panel.Height()), 1, colorKitKerb, false)
	}
	// Paint is light on the building's own dark ground floor, and the
	// billboard's panel is a light kit face, so its two lines are dark.
	ink := colorText
	if sign.Billboard() {
		ink = colorKitKerb
	}
	for _, line := range sign.Lines {
		g.sign(screen, line, ink)
	}
}

// droneHover is how far above the roofline a subagent's drone flies, in
// pixels at zoom 1. It is measured to the drone's own foot: since the
// atlas began deriving anchors from the mesh, a piece that never
// touches the ground is anchored at the bottom of itself rather than at
// the ground beneath it, which is 19 px further up at this zoom.
const droneHover = 31.0

// sign draws painted-on text: scaled with the map, and turned a quarter
// anticlockwise to run up a side when the sign is vertical.
func (g *Game) sign(screen *ebiten.Image, s ui.Sign, c color.NRGBA) {
	op := &text.DrawOptions{}
	op.GeoM.Scale(s.Scale, s.Scale)
	if s.Vertical {
		op.GeoM.Rotate(-math.Pi / 2)
	}
	// A board standing in the world is not square to the screen, so its
	// paint leans with it. Skew takes an angle; Lean is a gradient.
	if s.Lean != 0 {
		op.GeoM.Skew(0, math.Atan(s.Lean))
	}
	op.GeoM.Translate(s.At.X, s.At.Y)
	op.ColorScale.ScaleWithColor(c)
	op.Filter = ebiten.FilterLinear
	text.Draw(screen, s.Text, g.faces.Face(ui.Small), op)
}

// vacantPlot is a session with nothing typed into it yet: bare ground
// inside the plot's kerb, no building, no light, hoverable like one.
func (g *Game) vacantPlot(screen *ebiten.Image, cam *city.Camera, b *city.Building, selected bool) {
	plot := b.Rect.Inset(city.Tile * 0.15)
	fill := colorVacant
	if g.scene.Dimmed(b) {
		fill = colorVacantDim
	}
	g.poly(screen, cam, plot, fill)
	g.polyStroke(screen, cam, plot, 1, colorKitKerb)
	if selected {
		g.polyStroke(screen, cam, plot, 2, colorSelected)
	}
	g.noteHit(screenBounds(cam, plot), city.Hit{Building: b, District: g.scene.City().DistrictOf(b)})
}

// screenBounds is the screen box around a world rectangle's projection.
func screenBounds(cam *city.Camera, r city.Rect) city.Rect {
	var out city.Rect
	for i, corner := range cam.Corners(r) {
		p := corner.Add(cam.Offset).Scale(cam.Zoom)
		if i == 0 {
			out = city.Rect{Min: p, Max: p}
			continue
		}
		out.Min.X, out.Min.Y = math.Min(out.Min.X, p.X), math.Min(out.Min.Y, p.Y)
		out.Max.X, out.Max.Y = math.Max(out.Max.X, p.X), math.Max(out.Max.Y, p.Y)
	}
	return out
}

func (g *Game) noteHit(r city.Rect, hit city.Hit) {
	if r.Area() == 0 {
		return
	}
	g.mu.Lock()
	g.frameHits = append(g.frameHits, spriteHit{rect: r, hit: hit})
	g.mu.Unlock()
}

// drawIso is the isometric frame: ground, lines, district floors, then
// every building and landmark painted back to front.
func (g *Game) drawIso(screen *ebiten.Image, c *city.City, cam *city.Camera, hover city.Hit, selected *city.Building, width, height float64, seconds float64) {
	detailed := g.scene.Detailed()
	labels := g.labelsVisible()
	defer frames.start()()
	g.drawing.reset()
	defer func() {
		g.mu.Lock()
		g.hits = append(g.hits[:0], g.frameHits...)
		g.drawnOrder.copyFrom(&g.drawing)
		g.mu.Unlock()
	}()
	// The city under the traffic, composed once and blitted; then its
	// hover targets replayed, because the frame is drawn fresh even when
	// the layer is not.
	under := g.staticCity(c, cam, hover, width, height)
	g.mu.Lock()
	g.frameHits = append(g.frameHits[:0], g.static.hits...)
	g.mu.Unlock()
	g.blitBase(screen, under)
	g.overgrowth(screen, cam, false)
	g.agedLots(screen, cam, c)
	// Drones stand in the depth sort on their shadows' ground points,
	// and their shadows go on the ground first (item 89).
	shadows, drones := g.flights(&screen, c, cam, seconds)
	g.fadeAll(&screen, func() {
		g.powerLines(screen, c, cam, hover, seconds)
		for _, at := range shadows {
			droneShadow(screen, cam, at)
		}
	})
	items := drones
	var airborne []func()
	if c.Fountain.Area() > 0 {
		items = append(items, footprintAt(cam, c.Fountain, func() {
			g.fountain(screen, cam, c, seconds)
		}))
	}
	for _, l := range c.Lamps {
		if !g.scene.Scenery() {
			break
		}
		l := l
		items = append(items, footprintAt(cam, l.Rect(), func() {
			g.lamp(screen, cam, l, c.Night)
		}))
	}
	for _, d := range c.Districts {
		for _, b := range d.Buildings {
			b := b
			// The building's own work, its scaffold and hoist, is drawn
			// with it, so it takes the building's place in the sort
			// (item 89).
			items = append(items, buildingItem(cam, b.Rect, func() {
				g.isoBuilding(screen, cam, b, b == selected, detailed, seconds)
			}, func() {
				if g.kits != nil && !b.Vacant && !g.scene.Dimmed(b) {
					g.drawSkip(screen, cam, b, nil)
				}
				if detailed && g.kits != nil && !g.viewing() && !b.Vacant && !g.scene.Dimmed(b) {
					g.drawWork(screen, cam, b)
				}
				// The spire goes over the work: a segment is half a storey,
				// so the storey being built would hide the first of them.
				if g.kits != nil && !b.Vacant && !g.scene.Dimmed(b) {
					g.drawSpire(screen, cam, b)
				}
			}))
			items[len(items)-1].kept = g.keeps(d)
		}
	}
	for _, v := range g.scene.Voyages() {
		v := v
		items = append(items, standingAt(cam, v.At(g.scene.Clock()), func() {
			g.drawVoyage(screen, cam, v, seconds)
		}))
	}
	if !g.reduced {
		for _, h := range g.scene.Hauls() {
			h := h
			items = append(items, standingAt(cam, h.At(g.scene.Clock()), func() {
				g.drawHaul(screen, cam, h)
			}))
		}
	}
	for _, b := range c.Buildings() {
		if in, ok := g.scene.Inspector(b.Session.ID); ok && !b.Vacant && !g.scene.Dimmed(b) {
			b := b
			items = append(items, standingAt(cam, in.At, func() {
				g.drawInspector(screen, cam, b, in)
			}))
			items[len(items)-1].kept = g.keeps(c.DistrictOf(b))
		}
	}
	for _, m := range g.scene.Messages() {
		m := m
		items = append(items, standingAt(cam, m.At(g.scene.Clock()), func() {
			g.drawMessage(screen, cam, m)
		}))
	}
	for _, d := range g.scene.Deliveries() {
		d := d
		items = append(items, standingAt(cam, d.At(g.scene.Clock()), func() {
			g.drawDelivery(screen, cam, d)
		}))
	}
	for _, r := range g.scene.Ribbons() {
		r := r
		airborne = append(airborne, func() { g.drawRibbon(screen, cam, r) })
	}
	for _, k := range g.freight(c, seconds) {
		k := k
		items = append(items, standingAt(cam, k.at, func() {
			g.drawCarriage(screen, cam, k)
		}))
	}
	for _, car := range g.trafficCars(c, seconds) {
		car := car
		items = append(items, standingAt(cam, car.at, func() {
			g.drawCar(screen, cam, car)
		}))
		items[len(items)-1].kept = g.keepsRoad(car.road)
	}
	if c.Plant.Rect.Area() > 0 {
		items = append(items, footprintAt(cam, c.Plant.Rect, func() {
			g.isoLandmark(screen, cam, c.Plant.Rect, kitPlant, g.scenery(), city.Hit{Landmark: city.LandmarkPlant})
			g.letterLandmark(screen, cam, c, kitPlant)
		}))
		// The chimney shares the building's depth and is appended after
		// it. The sort is stable, so the slab is laid down first and the
		// chimney over it, foot on the roof plane instead of the
		// pavement. Order here is load-bearing: swap these two appends
		// and the slab paints over the chimney's base. Bug 39.
		perch := landmarkPerch(c.Plant.Rect, g.kitFootprintTurned(cam, kitPlant, landmarkTurn))
		roof := g.kitRoofLiftTurned(cam, kitPlant, landmarkTurn)
		items = append(items, footprintAt(cam, c.Plant.Rect, func() {
			// The fittings go on after the stack and over its cut
			// edge: the flashing's near rim is what makes the join a
			// curve instead of the straight line a cylinder cannot
			// meet a roof on. Bug 46.
			g.stackThrough(screen, cam, kitStack, perch, roof)
		}))
	}
	// The usage boats. They sort with the city like anything else, and
	// they are deliberately a different hull from the voyage tugs on
	// the same water — one river, two meanings, told apart by
	// silhouette and by what each says when pointed at. Item 49.
	//
	// Item 79: they drive between readings and sail in and out, so where
	// a boat is comes from the scene's track for it, and a river with no
	// boat on it has no markers either.
	if boats := g.scene.GaugeBoats(); len(boats) > 0 {
		if from, to, width, ok := riverRun(c); ok {
			age := g.usageAge()
			// The channel markers float with everything else, because
			// a buoy is a thing on the river rather than a mark on it.
			for _, m := range gaugeMarks(cam, from, to, width) {
				m := m
				items = append(items, standingAt(cam, m.At, func() {
					piece := kitBuoy
					if m.Limit {
						piece = kitBuoyLimit
					}
					g.kitSized(screen, cam, piece, 0, m.At, buoyExtent, nil)
				}))
			}
			var back, ahead float64
			if bandFrom, bandTo, _, ok := c.RiverBand(); ok {
				back, ahead = gaugeReach(cam, from, to, bandFrom, bandTo)
			}
			// One lane per boat, so two boats whose readings converge
			// are still side by side rather than on top of each other.
			// Bug 52.
			lanes := gaugeLanes(width, len(boats))
			facing := gaugeFacing(cam, from, to)
			for _, boat := range boats {
				gauge := boat.Gauge
				lane := 0.0
				if gauge.Lane >= 0 && gauge.Lane < len(lanes) {
					lane = lanes[gauge.Lane]
				}
				at := gaugeTrackAt(cam, from, to, lane, boat.Track, back, ahead)
				items = append(items, standingAt(cam, at, func() {
					r := g.drawGauge(screen, cam, at, gauge.Size, facing)
					g.noteHit(r, city.Hit{Gauge: &city.GaugeHit{Gauge: gauge, Age: age}})
				}))
			}
		}
	}
	// The refresh couriers. Not sessions and not gauges — a third
	// meaning on the same water, so it gets its own hull and its own
	// card. Item 56.
	for _, courier := range g.scene.Couriers() {
		courier := courier
		at := courier.At(g.scene.Clock())
		items = append(items, standingAt(cam, at, func() {
			dir := city.Point{X: courier.To.X - courier.From.X, Y: courier.To.Y - courier.From.Y}
			r := g.kitSized(screen, cam, kitCourier, bowTurn(unit(dir)), at, courierExtent, nil)
			g.noteHit(r, city.Hit{Courier: courier})
		}))
	}
	// A project's sign is an object on the ground, so it sorts with
	// everything else rather than being painted over the scene. Bug 44.
	for _, d := range c.Districts {
		d := d
		at, _, _, ok := g.monumentOf(cam, d)
		if !ok {
			continue
		}
		items = append(items, standingAt(cam, at, func() {
			g.districtMonument(screen, cam, d, c.Night)
		}))
		items[len(items)-1].kept = g.keeps(d)
	}
	for _, t := range c.Trees {
		if !g.scene.Scenery() {
			break
		}
		t := t
		items = append(items, standingAt(cam, t.At, func() {
			g.isoTree(screen, cam, t, g.treeTint(t))
		}))
	}
	for _, t := range c.Towers {
		t := t
		items = append(items, footprintAt(cam, t.Rect, func() {
			r := g.kit(screen, cam, kitTower, 0, t.Rect.Center(), towerTint(t, g.viewing(), g.unlitTower(t)))
			g.noteHit(r, city.Hit{Landmark: city.LandmarkTower, Tower: t})
			g.towerSign(screen, r, t.Server.Name)
		}))
	}
	if c.Library.Rect.Area() > 0 {
		items = append(items, footprintAt(cam, c.Library.Rect, func() {
			g.isoLandmark(screen, cam, c.Library.Rect, kitLibrary, g.scenery(), city.Hit{Landmark: city.LandmarkLibrary})
			g.letterLandmark(screen, cam, c, kitLibrary)
		}))
	}
	if c.Hall.Rect.Area() > 0 {
		items = append(items, footprintAt(cam, c.Hall.Rect, func() {
			g.isoLandmark(screen, cam, c.Hall.Rect, kitHall, g.scenery(), city.Hit{Landmark: city.LandmarkHall})
			g.letterLandmark(screen, cam, c, kitHall)
		}))
	}
	// Each district's board stands in the world, ordered with the
	// buildings around it (item 88).
	signed := map[*city.District]bool{}
	if !g.hidden {
		for _, d := range c.Districts {
			at := d.Sign().At
			items = append(items, footprintAt(cam, city.RectAt(at.X-1, at.Y-1, 2, 2), func() {
				signed[d] = g.districtSign(screen, cam, d)
			}))
			items[len(items)-1].kept = g.keeps(d)
		}
	}
	fade := g.fadeUnkept(&screen, items, airborne)
	for _, draw := range layered(cam, items, airborne) {
		draw()
	}
	fade.done()
	if hover.Building != nil && hover.Building != selected {
		g.polyStroke(screen, cam, hover.Building.Rect, 1.5, colorHighlight)
	}
	if c.Night {
		vector.FillRect(screen, 0, 0, float32(width), float32(height), colorNightOverlay, false)
		g.nightLights(screen, cam, c)
	}
	g.fadeAll(&screen, func() { g.groupLines(screen, cam, false) })
	g.groupLines(screen, cam, true)
	for _, d := range c.Districts {
		// The sign stands in the district; the plate is for when it is
		// too small to read, and for the district under the pointer.
		if g.districtLabelVisible(d) && (!signed[d] || hover.District == d) {
			g.queueDistrictPlate(d)
		}
	}
	g.queueRoadTags(c, cam)
	g.queueSightings(width, height)
	if labels {
		g.queueLandmarkLabels(cam, c, hover)
	}
	g.drawMapLabels(screen)
	if labels {
		g.towerPlate(screen, cam, c, hover)
	}
}

// queueLandmarkLabels asks for the hovered landmark's plate. Each
// landmark's name is painted on its wall, so the plate is for the one
// under the pointer only (item 99; Aria, 2026-10-19).
func (g *Game) queueLandmarkLabels(cam *city.Camera, c *city.City, hover city.Hit) {
	if c.Plant.Rect.Area() > 0 && hover.Landmark == city.LandmarkPlant {
		g.queueLandmarkLabel(cam, c.Plant.Rect, kitPlant, "power plant")
	}
	if c.Library.Rect.Area() > 0 && hover.Landmark == city.LandmarkLibrary {
		g.queueLandmarkLabel(cam, c.Library.Rect, kitLibrary, "library")
	}
	if c.Hall.Rect.Area() > 0 && hover.Landmark == city.LandmarkHall {
		g.queueLandmarkLabel(cam, c.Hall.Rect, kitHall, "city hall")
	}
}

// towerPlate names the hovered tower. A tower's name is signage on the
// tower itself; the plate is for the hovered one only, so the ridge
// never piles up with text.
func (g *Game) towerPlate(screen *ebiten.Image, cam *city.Camera, c *city.City, hover city.Hit) {
	for _, t := range c.Towers {
		if hover.Tower != t {
			continue
		}
		top, w := footprint(cam, t.Rect)
		name := t.Server.Name
		wText, h := g.measure(name)
		g.floorLabel(screen, city.Point{X: top.X - w/2 - wText - g.theme.Px(8), Y: top.Y + w/4 - h/2}, name, colorDim)
	}
}

// roadTile names the road pack's sprite for a cell's joins. The pack's
// compass is one step round from the world axes: its N is the tile's
// top-left edge (world -x), E the top-right (world -y), S the bottom-right
// (world +x) and W the bottom-left (world +y).
func roadTile(mask int) string {
	return packTile("road", "end", "crossroad", mask)
}

// riverTile names the river sprite for a river cell's joins; a source or
// mouth with one join runs straight along its axis.
func riverTile(mask int) string {
	switch mask {
	case city.DirN, city.DirS:
		mask = city.DirN | city.DirS
	case city.DirE, city.DirW:
		mask = city.DirE | city.DirW
	}
	return packTile("river", "river", "river", mask)
}

// lakeTile names the water sprite for a lake cell by the sides it meets
// land on, in the pack's rotated compass.
func lakeTile(land int) string {
	if land == 0 {
		return "water"
	}
	return packTile("water", "water", "water", land)
}

// bridgeTile is the bridge for a straight street across the river, or ""
// when the street is not straight there.
func bridgeTile(mask int) string {
	switch mask {
	case city.DirN | city.DirS:
		return "bridgeEW"
	case city.DirE | city.DirW:
		return "bridgeNS"
	}
	return ""
}

func packTile(two, one, many string, mask int) string {
	letters := ""
	for _, side := range []struct {
		dir    int
		letter string
	}{{city.DirW, "N"}, {city.DirN, "E"}, {city.DirE, "S"}, {city.DirS, "W"}} {
		if mask&side.dir != 0 {
			letters += side.letter
		}
	}
	switch len(letters) {
	case 1:
		return one + letters
	case 2:
		return two + letters
	case 3:
		return many + letters
	case 4:
		return many
	}
	return ""
}

// streets lays the autotiled road cells over the grass and signs each
// road at the middle of its path.
func (g *Game) streets(screen *ebiten.Image, c *city.City, cam *city.Camera, hover city.Hit, labels bool) {
	if g.kits == nil {
		g.roadLines(screen, c, cam, hover, labels)
		return
	}
	lit := g.litGround(c)
	for _, sc := range c.StreetCells {
		name, turn := roadPiece(sc.Mask)
		g.kitGround(screen, cam, name, turn, sc.Cell.Center(), groundTint(lit, sc.Cell))
	}
	g.fadeAll(&screen, func() { g.bridge(screen, c, cam) })
	g.streetSigns(screen, c, cam, hover, labels)
}

// overgrowth draws each road cell whose connection has ceased fading
// into the belt beneath it. It changes every frame while it runs, so it
// is drawn over the static layer rather than composed into it. Item 85.
func (g *Game) overgrowth(screen *ebiten.Image, cam *city.Camera, flat bool) {
	for _, o := range g.scene.Overgrowth() {
		if flat || g.kits == nil {
			col := g.flat(colorStreet)
			col.A = uint8(float64(col.A) * (1 - o.Progress))
			g.rect(screen, cam, o.Cell.Rect(), col)
			continue
		}
		var tint ebiten.ColorScale
		tint.ScaleAlpha(float32(1 - o.Progress))
		name, turn := roadPiece(o.Mask)
		g.kitGround(screen, cam, name, turn, o.Cell.Center(), &tint)
	}
	g.overgrownPads(screen, g.scene.City(), cam, flat)
}

// streetSigns highlights the hovered street and signs each one at the
// middle of its path, in either view.
func (g *Game) streetSigns(screen *ebiten.Image, c *city.City, cam *city.Camera, hover city.Hit, labels bool) {
	// Traffic paints the carrier: each avenue in the colour of what it
	// is carrying, so the busy route between two repos reads before you
	// have counted a single car. The grid itself stays in every view —
	// it is the city's shape, not one of its networks — and this is only
	// the load on it. It rides in the static layer, so it costs one
	// compose rather than a frame.
	roads := c.Roads
	for i := range c.Streets {
		street := &c.Streets[i]
		if street.Road != nil {
			if t := g.scene.RoadTint(*street.Road, roads); t.Known {
				for k := 1; k < len(street.Path); k++ {
					g.line(screen, cam, street.Path[k-1], street.Path[k], float32(1+3*t.Value), ui.Sequential.At(t.Value))
				}
			}
		}
		if hover.Road == street.Road || (street.Road != nil && g.lit.HasRoad(street.Road)) {
			for k := 1; k < len(street.Path); k++ {
				g.line(screen, cam, street.Path[k-1], street.Path[k], 2, colorHighlight)
			}
		}
		if labels {
			mid := street.Path[len(street.Path)/2]
			g.floorLabel(screen, cam.WorldToScreen(mid).Add(city.Point{X: g.theme.Px(6), Y: -g.lineHeight() - g.theme.Px(6)}), street.Road.Label(), colorDim)
		}
	}
}

// Traffic: a car per working traveller, driving the street's path kerb
// to kerb and back.
const carSpeed = 60.0 // world units per second

type car struct {
	at    city.Point
	model string
	turn  int
	// road is the pair of repos this car's traffic is between. A car
	// cannot say that by where it is — the street is routed on the grid
	// and runs along whichever districts happen to be adjacent — so it
	// has to carry it.
	road *city.RoadLine
}

func (g *Game) cars(c *city.City, seconds float64) []car {
	var out []car
	for i := range c.Streets {
		street := &c.Streets[i]
		length := pathLength(street.Path)
		if length <= 0 {
			continue
		}
		// One car per traveller working on the connection, and none while
		// nobody is: the road stays, the cars do not. Item 85.
		count := street.Road.Active
		for k := 0; k < count; k++ {
			// Cars go there and back along the path.
			phase := math.Mod(seconds*carSpeed/(2*length)+float64(k)/float64(count)+float64(i)*0.37, 1)
			t := phase * 2
			forward := t <= 1
			if !forward {
				t = 2 - t
			}
			at, dir := pointAlong(street.Path, t*length)
			if !forward {
				dir = city.Point{X: -dir.X, Y: -dir.Y}
			}
			out = append(out, car{at: at, model: kitCars[(i*len(kitCars)/2+k)%len(kitCars)], turn: carTurn(dir), road: street.Road})
		}
	}
	return out
}

func pathLength(path []city.Point) float64 {
	total := 0.0
	for i := 1; i < len(path); i++ {
		total += math.Hypot(path[i].X-path[i-1].X, path[i].Y-path[i-1].Y)
	}
	return total
}

// pointAlong is the point a distance along a polyline and the direction
// of the segment it is on.
func pointAlong(path []city.Point, dist float64) (city.Point, city.Point) {
	for i := 1; i < len(path); i++ {
		a, b := path[i-1], path[i]
		seg := math.Hypot(b.X-a.X, b.Y-a.Y)
		if dist <= seg || i == len(path)-1 {
			u := math.Min(1, dist/math.Max(seg, 1e-9))
			return city.Point{X: a.X + (b.X-a.X)*u, Y: a.Y + (b.Y-a.Y)*u}, city.Point{X: b.X - a.X, Y: b.Y - a.Y}
		}
		dist -= seg
	}
	return path[0], city.Point{X: 1}
}

// camps ties each team's members to their lead with a dashed line in
// the accent, so a crew reads as one across districts.
func (g *Game) camps(screen *ebiten.Image, c *city.City, cam *city.Camera) {
	for _, camp := range c.Camps {
		if camp.Lead == nil {
			continue
		}
		from := cam.WorldToScreen(camp.Lead.Rect.Center())
		for _, m := range camp.Members {
			to := cam.WorldToScreen(m.Rect.Center())
			dashed(screen, from, to, math.Max(1, 1.5*cam.Zoom), 6*cam.Zoom, g.theme.Palette.Accent)
		}
	}
}

// dashed strokes a line as dashes.
func dashed(screen *ebiten.Image, a, b city.Point, width, dash float64, col color.NRGBA) {
	length := math.Hypot(b.X-a.X, b.Y-a.Y)
	if length == 0 || dash <= 0 {
		return
	}
	ux, uy := (b.X-a.X)/length, (b.Y-a.Y)/length
	for d := 0.0; d < length; d += 2 * dash {
		end := math.Min(length, d+dash)
		vector.StrokeLine(screen, float32(a.X+ux*d), float32(a.Y+uy*d), float32(a.X+ux*end), float32(a.Y+uy*end), float32(width), col, true)
	}
}

// celebrate is the one-shot show for a merge: a green ring swelling
// out of the roof and a few sparks rising, over CelebrateFor.
func (g *Game) celebrate(screen *ebiten.Image, top city.Point, width float64, p float64) {
	green := g.theme.Palette.Merged
	fade := uint8(255 * (1 - p))
	ring := color.NRGBA{green.R, green.G, green.B, fade}
	radius := width * (0.3 + 1.2*p)
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(ring)
	var path vector.Path
	path.Arc(float32(top.X), float32(top.Y), float32(radius), 0, 2*math.Pi, vector.Clockwise)
	vector.StrokePath(screen, &path, &vector.StrokeOptions{Width: float32(math.Max(1.5, 3*(1-p)))}, op)
	for i := 0; i < 6; i++ {
		a := float64(i) * math.Pi / 3
		rise := width * (0.2 + 0.9*p)
		vector.FillCircle(screen, float32(top.X+math.Cos(a)*width*0.35*(0.5+p)), float32(top.Y-rise*math.Abs(math.Sin(a+1))), float32(2.5*(1-p)+1), ring, true)
	}
}

// carTurn is a kit car's turn for the way it drives: the kit's cars are
// modelled nose along +y, so that is turn 0.
func carTurn(d city.Point) int {
	switch {
	case math.Abs(d.Y) >= math.Abs(d.X) && d.Y >= 0:
		return 0
	case math.Abs(d.Y) >= math.Abs(d.X):
		return 180
	case d.X >= 0:
		return 270
	default:
		return 90
	}
}

func (g *Game) drawCar(screen *ebiten.Image, cam *city.Camera, car car) {
	r := g.kit(screen, cam, car.model, car.turn, car.at, nil)
	// A road's tag steps off the car rather than covering it (item 108).
	g.queueLabel(pendingLabel{mapLabel: mapLabel{Kind: labelCar, Rect: r, Anchor: r.Center()}})
	if car.road == nil {
		return
	}
	from, to := "", ""
	if car.road.From != nil {
		from = car.road.From.Name
	}
	if car.road.To != nil {
		to = car.road.To.Name
	}
	g.noteHit(r, city.Hit{Car: &city.CarHit{Road: car.road, From: from, To: to}})
}

// Night: the wash dims everything, then every building with a session
// awake in it glows from its windows, so at night the map reads as
// "which lights are on".
var (
	colorWindowGlow = color.NRGBA{0xff, 0xc8, 0x70, 0x2c}
	colorWindowCore = color.NRGBA{0xff, 0xe0, 0xa0, 0x30}
	colorPlantGlow  = color.NRGBA{0xf0, 0xb4, 0x4c, 0x40}
)

// At fit a lamp is a pixel and a window a fraction of one, so every
// light keeps a floor in screen pixels and gets brighter the further
// out the view is: what matters at fit is which lights are on, not
// their size.
const (
	minLampGlow   = 3.0
	minWindowGlow = 2.5
	minPlantGlow  = 14.0
	glowBoostZoom = 0.6
	glowBoostMax  = 2.5
)

// lit is a glow radius with a floor in pixels.
func lit(radius, floor float64) float64 {
	return math.Max(radius, floor)
}

// windowGlow is lit for a window, held to the storey it lights.
//
// The discs are additive — two that overlap are brighter than either,
// and a column of them saturates to white. That is deliberate at fit,
// where what matters is which lights are on rather than their size. It
// stops being deliberate as the view comes in: the radius grows with the
// zoom while the storeys between the discs do not grow faster, so a
// five-storey building came to wear five merged discs and disappeared
// behind its own windows.
//
// So the radius is capped at half a storey, which is what keeps two of
// them apart, and the floor is itself held to the storey — a floor that
// is taller than the thing it lights is not a floor, it is the whole
// building.
func windowGlow(radius, floor, storey float64) float64 {
	if storey <= 0 {
		return math.Max(radius, floor)
	}
	return math.Min(math.Max(radius, math.Min(floor, storey/2)), storey/2)
}

// boost brightens a light's alpha as the view zooms out past
// glowBoostZoom, up to glowBoostMax times.
func boost(c color.NRGBA, zoom float64) color.NRGBA {
	if zoom >= glowBoostZoom {
		return c
	}
	k := math.Min(glowBoostMax, glowBoostZoom/zoom)
	c.A = uint8(math.Min(255, float64(c.A)*k))
	return c
}

func (g *Game) nightLights(screen *ebiten.Image, cam *city.Camera, c *city.City) {
	for _, b := range c.Buildings() {
		if !b.Lit || b.Blinds {
			continue
		}
		foot := g.footOf(cam, b)
		size := g.kitSize(cam, buildingPiece(b))
		w, h := float64(size.X), float64(size.Y)
		storeys := max(1, int(h/(38*cam.Zoom)))
		storey := h * 0.7 / float64(storeys)
		for i := 0; i < storeys; i++ {
			y := foot.Y - h*0.15 - (h*0.7)*(float64(i)+0.5)/float64(storeys)
			side := windowGlow(w*0.16, minWindowGlow, storey)
			core := windowGlow(w*0.1, minWindowGlow*0.6, storey)
			glow(screen, city.Point{X: foot.X - w*0.2, Y: y}, side, boost(colorWindowGlow, cam.Zoom))
			glow(screen, city.Point{X: foot.X + w*0.2, Y: y}, side, boost(colorWindowGlow, cam.Zoom))
			glow(screen, city.Point{X: foot.X, Y: y}, core, boost(colorWindowCore, cam.Zoom))
		}
	}
	if c.Plant.Rect.Area() > 0 {
		foot := cam.WorldToScreen(c.Plant.Rect.Center())
		size := g.kitSizeTurned(cam, kitPlant, landmarkTurn)
		glow(screen, city.Point{X: foot.X, Y: foot.Y - float64(size.Y)*0.6}, lit(float64(size.X)*0.22, minPlantGlow), boost(colorPlantGlow, cam.Zoom))
	}
}

// glow is a soft additive disc: light, not paint.
func glow(screen *ebiten.Image, at city.Point, radius float64, col color.NRGBA) {
	var path vector.Path
	path.Arc(float32(at.X), float32(at.Y), float32(radius), 0, 2*math.Pi, vector.Clockwise)
	path.Close()
	op := &vector.DrawPathOptions{AntiAlias: true, Blend: ebiten.BlendLighter}
	op.ColorScale.ScaleWithColor(col)
	vector.FillPath(screen, &path, nil, op)
}

// towerTint is the colour a tower carries for its own reasons: warm when
// something is calling that server, cold when nothing is.
//
// Under a view it carries none. A view is subtractive, and the recede
// happens on the nil-tint path in drawSprite, so an object that always
// hands over a scale of its own is an object a view can never drain —
// which is how thirteen bright water towers went on standing over a
// city that had stepped back.
func towerTint(t *city.Tower, viewing, unlit bool) *ebiten.ColorScale {
	if unlit {
		return dimmed()
	}
	if viewing {
		return nil
	}
	tint := &ebiten.ColorScale{}
	if t.Server.Calls == 0 {
		tint.SetR(0.6)
		tint.SetG(0.6)
		tint.SetB(0.65)
	} else {
		tint.SetR(0.7)
		tint.SetG(1)
		tint.SetB(0.95)
	}
	return tint
}

// trafficCars and freight are the two mover networks behind their views:
// the cars are what Traffic is about and the train is what Spend is,
// and Attention runs both, because each explains itself on hover. Gating here rather than at each draw
// keeps the cost out of the sorted list as well as off the screen —
// nothing is placed, so nothing is depth-sorted.
func (g *Game) trafficCars(c *city.City, seconds float64) []car {
	if !g.shows(city.NetworkTraffic) {
		return nil
	}
	return g.cars(c, seconds)
}

func (g *Game) freight(c *city.City, seconds float64) []carriage {
	if !g.shows(city.NetworkFreight) {
		return nil
	}
	return g.carriages(c, seconds)
}

// plantStackPerch is where the plant's chimney stands: the same world
// point the plant's own sprite is drawn at.
//
// Bug 39. It used to be one tile in from the footprint's near corner,
// which is not on the building at all. The plant's rect is a 7.5 x 4
// tile reservation and isoLandmark draws building-a at natural size —
// a little over 1.6 tiles wide — centred in it, so a point a tile in
// from Max is most of three tiles clear of the slab, out on the open
// plaza. That is where the chimney was standing, and why swapping
// chimney-large for chimney-medium changed how the error looked without
// touching what it was.
//
// isoLandmark draws the building at r.Center(); the chimney reads the
// same expression, so the two cannot be moved apart.
func plantStackPerch(plant city.Rect, side float64) city.Point {
	at := plant.Center()
	off := roofPerch(side)
	return city.Point{X: at.X + off.X, Y: at.Y + off.Y}
}

// footprintSide is how wide a piece's footprint is in world units, from
// the width of its sprite in the atlas.
//
// The unit here has bitten once already: one atlas cell is a
// BuildingSize square, which is three city tiles, so a 431 px sprite at
// the zoom-2 cut is a 78.4-unit square and not "1.6 tiles" of anything.
func footprintSide(spriteW, atlasZoom float64) float64 {
	return spriteW / (2 * city.IsoScale * atlasZoom)
}

// Where the plant's chimney stands on its roof, as a fraction of the
// building's own footprint. Read off building-a's sprite at heading 0
// and turned back into world units through the projection.
//
// It used to sit between the duct's two blocks, which was fine while
// the chimney was bare. Bug 47: the curb round the join has to be at
// least √2 times the pipe's radius to work at all, so it is wider than
// the duct is clear, and the clearance has to come from moving the
// chimney rather than from shrinking the curb — a curb narrower than
// that cannot cover the sprite's square corners at any height. The
// perch is now the open ground north-east of the roof's centre, clear
// of the tank at x<190, the duct below y=128 and the small pipes at
// x>310.
//
// This is a *world* offset, and that is what makes it safe. Bug 39
// claimed an off-centre perch would walk across the roof as the camera
// turns. That was wrong: the building does not turn, the camera does,
// so a world offset is rigidly attached to the roof and the chimney
// stays on the same physical spot from all four headings. It is a
// fraction of the *sprite box* that would walk, because the same
// fraction is a different physical point at each heading.
const (
	roofPerchEast  = 0.039
	roofPerchSouth = -0.132
)

func roofPerch(side float64) city.Point {
	return city.Point{X: side * roofPerchEast, Y: side * roofPerchSouth}
}

// sunkRows is how much of a piece is inside the building it rises
// through: everything below the roof plane, in the atlas's own pixels.
// cut is the roof's height above the piece's ground point on screen,
// and the rows below the anchor are the piece's own base.
func sunkRows(cut, scale, height, anchorY float64) float64 {
	return cut/scale + (height - anchorY)
}

// roofLift is how far above a piece's ground anchor its roof plane sits,
// in the atlas's own pixels.
//
// A kit sprite's box is exactly as wide as the piece's base diamond, so
// in this 2:1 projection the diamond's screen height is w/2 and its
// centre lies w/4 below the sprite's topmost pixel — which for a box of
// a building is the roof's far corner. The anchor's ay is the whole
// height above the ground point, so the roof plane is ay - w/4.
// building-a checks out: 431 px is 1.63 tiles against the 264 px tile,
// and 431/2 is the base diamond's screen height to the pixel.
func roofLift(w, ay float64) float64 {
	return math.Max(0, ay-w/4)
}

// stackDepth is the depth the plant's chimney sorts at: the building's,
// not the chimney's own.
//
// This reverses bug 23 deliberately. That gave the stack its own ground
// depth, which is right for something free-standing and wrong for a roof
// feature — DepthOf takes a footprint's back-most corner while Depth of
// the old perch was near its front-most, so the chimney sorted in front
// of the slab and was painted over it. A chimney belongs to its
// building's depth.
func stackDepth(cam *city.Camera, plant city.Rect) float64 {
	return cam.DepthOf(plant)
}

var (
	colorLotWear  = color.NRGBA{0x4a, 0x43, 0x38, 0xff}
	colorLotMoss  = color.NRGBA{0x48, 0x56, 0x34, 0xff}
	colorLotCrack = color.NRGBA{0x2e, 0x2a, 0x24, 0xff}
)

// dressing is the building's lot for this frame. Under reduced motion a
// mowing is a cross-fade of the whole growth rather than a clearing.
func (g *Game) dressing(b *city.Building) lot {
	mowing := g.scene.Mowing(b.Session.ID)
	if g.reduced && mowing >= 0 {
		d := dressLot(b, 0)
		g.weedFade = 1 - mowing
		return d
	}
	g.weedFade = 1
	return dressLot(b, mowing)
}

// agedLots paints every stopped building's lot on the ground, under
// every building, so nothing that ages a lot is ever drawn over a wall:
// the worn paving, its cracks, and the growth in them, at the base of
// the walls and in the yard. Item 86.
func (g *Game) agedLots(screen *ebiten.Image, cam *city.Camera, c *city.City) {
	width := float32(math.Max(1, 1.2*cam.Zoom))
	for _, d := range c.Districts {
		ground := dressDistrict(d)
		if ground.Wear == 0 {
			continue
		}
		stain := colorLotWear
		if ground.Mossy {
			stain = colorLotMoss
		}
		stain.A = uint8(255 * ground.Wear)
		g.poly(screen, cam, d.Rect, stain)
		for _, crack := range ground.Cracks {
			for i := 1; i < len(crack); i++ {
				g.line(screen, cam, crack[i-1], crack[i], width, colorLotCrack)
			}
		}
		for _, w := range ground.Weeds {
			g.kit(screen, cam, w.Piece, 0, w.At, nil)
		}
	}
	for _, b := range c.Buildings() {
		if g.scene.Dimmed(b) {
			continue
		}
		d := g.dressing(b)
		if d.Wear > 0 {
			stain := colorLotWear
			if b.Overgrown {
				stain = colorLotMoss
			}
			stain.A = uint8(255 * d.Wear)
			g.poly(screen, cam, lotRect(b), stain)
		}
		width := float32(math.Max(1, 1.2*cam.Zoom))
		for _, crack := range d.Cracks {
			for i := 1; i < len(crack); i++ {
				g.line(screen, cam, crack[i-1], crack[i], width, colorLotCrack)
			}
		}
		if len(d.Weeds) == 0 {
			continue
		}
		fade := &ebiten.ColorScale{}
		fade.ScaleAlpha(float32(g.weedFade))
		weeds := append([]weed(nil), d.Weeds...)
		sort.SliceStable(weeds, func(i, j int) bool { return weeds[i].At.X+weeds[i].At.Y < weeds[j].At.X+weeds[j].At.Y })
		for _, w := range weeds {
			g.kit(screen, cam, w.Piece, 0, w.At, fade)
		}
	}
}

// blindsTint is a stopped building's: lights off, blinds down.
func blindsTint() *ebiten.ColorScale {
	tint := &ebiten.ColorScale{}
	tint.SetR(0.7)
	tint.SetG(0.7)
	tint.SetB(0.72)
	return tint
}

const kitTruck = "car-kit/truck"

// drawHaul draws a truck carrying a packed-up session's crate between its
// lot and the depot, facing the way it drives (item 86).
func (g *Game) drawHaul(screen *ebiten.Image, cam *city.Camera, h city.Haul) {
	turn := carTurn(h.To.Sub(h.From))
	at := h.At(g.scene.Clock())
	g.kit(screen, cam, kitTruck, turn, at, nil)
	g.kitScaled(screen, cam, crates[h.Hue%len(crates)], turn, at, 0.4, nil)
}
