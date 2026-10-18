package render

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// The building's own work. Item 89.
//
// While a session is mid-turn the storey being built is scaffolded on its
// roof with work lights, in slice 2's construction strokes, one storey
// high on the building's own footprint (item 109), and each tool call
// runs a load up a hoist on the building's outside front corner to it.
// When the turn ends the scaffold comes down. At still a load is a flash
// of the work lights.

var (
	colorWorkLight = color.NRGBA{0xff, 0xd8, 0x60, 0xff}
	colorLoad      = color.NRGBA{0xc8, 0x8a, 0x3c, 0xff}
	colorHoist     = color.NRGBA{0x3a, 0x34, 0x2c, 0xff}
)

// buildingItem is a building in the depth sort: its sprite, then its
// work on top of it, so the scaffold and the hoist take the building's
// own place and a nearer building is drawn over both.
func buildingItem(cam *city.Camera, r city.Rect, sprite, work func()) drawable {
	return footprintAt(cam, r, func() {
		sprite()
		work()
	})
}

// frontOf is a footprint's corner nearest the viewer on screen, the one
// lowest down, and the corners either side of it.
func frontOf(lot [4]city.Point) (city.Point, [2]city.Point) {
	best := 0
	for i, p := range lot {
		if p.Y > lot[best].Y {
			best = i
		}
	}
	return lot[best], [2]city.Point{lot[(best+3)%4], lot[(best+1)%4]}
}

// storeyOf is how high the roof scaffold stands on a piece the pipeline
// gives no body: a storey of a building whose roof is roofLift up.
func storeyOf(roofLift float64) float64 { return roofLift * 0.3 }

// kindPlinth is the plinth the city's own buildings stand on, in kit
// units, as tools/render-sprites/buildings.py cuts them.
const kindPlinth = 0.03

// workSite is where a building's work stands on screen: its own
// footprint, how far up its walls end, and how tall a storey is.
type workSite struct {
	lot     [4]city.Point
	wallTop float64
	storey  float64
}

// workFrame is the building's own footprint and wall top for its work,
// from the body the pipeline wrote into the atlas; a piece without one
// works on its lot, at the roof its sprite shows.
func (g *Game) workFrame(cam *city.Camera, b *city.Building) (workSite, bool) {
	name := buildingPiece(b)
	if g.kits == nil {
		return workSite{}, false
	}
	atlas := g.kits.pick(cam.Zoom)
	if atlas == nil {
		return workSite{}, false
	}
	sprite, ok := atlas.Sprite(name, cam.Heading)
	if !ok {
		return workSite{}, false
	}
	if sprite.Body.W <= 0 {
		lift := g.kitRoofLift(cam, name)
		return workSite{lot: lotCorners(cam, b.Rect), wallTop: lift, storey: storeyOf(lift)}, lift > 0
	}
	c := b.Rect.Center()
	c.X += sprite.Body.X * city.CellSize
	hw, hd := sprite.Body.W*city.CellSize/2, sprite.Body.D*city.CellSize/2
	var lot [4]city.Point
	for i, p := range []city.Point{{X: c.X - hw, Y: c.Y - hd}, {X: c.X + hw, Y: c.Y - hd}, {X: c.X + hw, Y: c.Y + hd}, {X: c.X - hw, Y: c.Y + hd}} {
		lot[i] = cam.WorldToScreen(p)
	}
	lift := g.courseLift(cam, b)
	for i := range lot {
		lot[i].Y -= lift
	}
	storey := storeyHeight(cam.Zoom)
	n := min(max(b.Session.Storeys, 1), 5)
	return workSite{lot: lot, wallTop: (kindPlinth/kindStorey + float64(n)) * storey, storey: storey}, true
}

// scaffoldGrid is where the scaffold's poles stand across a roof, as
// shares of the footprint each way: the edges and the middle, so the
// scaffold fills the roof the way a storey going up does.
var scaffoldGrid = []float64{0.12, 0.5, 0.88}

// scaffoldSpans is how many planks one level of the grid lays: every pole
// joined to the next along each way.
var scaffoldSpans = 2 * len(scaffoldGrid) * (len(scaffoldGrid) - 1)

// onRoof is the point u and v of the way across a footprint lot, north,
// east, south, west, lifted roofLift to its roof.
func onRoof(lot [4]city.Point, roofLift, u, v float64) city.Point {
	n, e, s, w := lot[0], lot[1], lot[2], lot[3]
	return city.Point{
		X: (1-u)*(1-v)*n.X + u*(1-v)*e.X + u*v*s.X + (1-u)*v*w.X,
		Y: (1-u)*(1-v)*n.Y + u*(1-v)*e.Y + u*v*s.Y + (1-u)*v*w.Y - roofLift,
	}
}

// roofScaffold is the scaffold on a roof roofLift above a footprint lot
// on screen, up of the way raised: a grid of stout poles across the
// roof, and two levels of planks spanning between them, halfway and at
// the top.
func roofScaffold(lot [4]city.Point, roofLift, storey, up float64) []segment {
	if up <= 0 {
		return nil
	}
	h := storey * up
	pole := max(2, roofLift*0.035)
	plank := max(2, roofLift*0.05)
	first, last := scaffoldGrid[0], scaffoldGrid[len(scaffoldGrid)-1]
	edge := func(x float64) bool { return x == first || x == last }
	var out []segment
	// Eight poles: the corners and the middle of each edge, none in the
	// middle of the roof, where in this view it would stand in line with
	// the front corner and cross the boards.
	middle := onRoof(lot, roofLift, 0.5, 0.5)
	for _, u := range scaffoldGrid {
		for _, v := range scaffoldGrid {
			if !edge(u) && !edge(v) {
				continue
			}
			foot := onRoof(lot, roofLift, u, v)
			out = append(out, segment{
				a: foot, b: city.Point{X: foot.X, Y: foot.Y - h}, width: pole,
				pole: true, corner: edge(u) && edge(v), back: foot.Y < middle.Y-1e-9,
			})
		}
	}
	// A diagonal across each bay of the two faces toward the camera,
	// which is what makes it read as scaffolding rather than a rail.
	type face struct {
		at func(t float64) (float64, float64)
	}
	faces := []face{
		{func(t float64) (float64, float64) { return t, first }},
		{func(t float64) (float64, float64) { return t, last }},
		{func(t float64) (float64, float64) { return first, t }},
		{func(t float64) (float64, float64) { return last, t }},
	}
	nearness := func(f face) float64 {
		u, v := f.at(0.5)
		return onRoof(lot, roofLift, u, v).Y
	}
	for pick := 0; pick < 2; pick++ {
		best := pick
		for i := pick; i < len(faces); i++ {
			if nearness(faces[i]) > nearness(faces[best]) {
				best = i
			}
		}
		faces[pick], faces[best] = faces[best], faces[pick]
		f := faces[pick]
		for k := 0; k+1 < len(scaffoldGrid); k++ {
			u0, v0 := f.at(scaffoldGrid[k])
			u1, v1 := f.at(scaffoldGrid[k+1])
			from := onRoof(lot, roofLift, u0, v0)
			to := onRoof(lot, roofLift, u1, v1)
			out = append(out, segment{a: from, b: city.Point{X: to.X, Y: to.Y - h}, width: pole * 0.8, brace: true})
		}
	}
	for _, level := range []float64{0.5, 1} {
		lift := func(p city.Point) city.Point { return city.Point{X: p.X, Y: p.Y - h*level} }
		for i := range scaffoldGrid {
			for k := 0; k+1 < len(scaffoldGrid); k++ {
				a, b := scaffoldGrid[k], scaffoldGrid[k+1]
				g := scaffoldGrid[i]
				out = append(out,
					segment{a: lift(onRoof(lot, roofLift, a, g)), b: lift(onRoof(lot, roofLift, b, g)), width: plank, plank: true},
					segment{a: lift(onRoof(lot, roofLift, g, a)), b: lift(onRoof(lot, roofLift, g, b)), width: plank, plank: true},
				)
			}
		}
	}
	return out
}

// scaffoldOrder is a scaffold's strokes in the order they are drawn,
// back to front: the poles on its far side, then the boards, then the
// poles and braces in front, so the wood passes in front of what is
// behind it.
func scaffoldOrder(lines []segment) []segment {
	var back, boards, front []segment
	for _, s := range lines {
		switch {
		case s.pole && s.back:
			back = append(back, s)
		case s.plank:
			boards = append(boards, s)
		default:
			front = append(front, s)
		}
	}
	return append(append(back, boards...), front...)
}

// workLights is where the work lights hang: on the scaffold's corner
// poles, but not the back one, which the building hides.
func workLights(lines []segment) []city.Point {
	var corners []segment
	for _, s := range lines {
		if s.pole && s.corner {
			corners = append(corners, s)
		}
	}
	back := -1
	for i, s := range corners {
		if back < 0 || s.a.Y < corners[back].a.Y {
			back = i
		}
	}
	var out []city.Point
	for i, s := range corners {
		if i != back {
			out = append(out, s.b)
		}
	}
	return out
}

// roofItem is a construction item on a scaffolded roof deck: the column
// of the next storey's frame, rising.
type roofItem struct {
	kind string
	at   city.Point
	size float64
}

// roofItems is what stands on a roof's deck while it is scaffolded, up of
// the way raised, and nothing once the scaffold is down.
func roofItems(lot [4]city.Point, roofLift, up float64) []roofItem {
	if up <= 0 {
		return nil
	}
	size := max(4, roofLift*0.1) * up
	return []roofItem{{kind: "column", at: onRoof(lot, roofLift, 0.5, 0.3), size: size}}
}

// crateSize is how big a load's crate is drawn on a building whose roof
// is roofLift up: big enough to read at the opening zoom.
func crateSize(roofLift float64) float64 { return max(6, roofLift*0.12) }

// hoistAt is where a load is p of the way up the hoist on the outside
// front corner: at the foot at the start, at the roof's scaffold at the
// end. It runs just beside the corner, so the scaffold's front pole is
// never covered by it.
func hoistAt(lot [4]city.Point, roofLift, p float64) city.Point {
	front, _ := frontOf(lot)
	return city.Point{X: front.X + max(4, roofLift*0.09), Y: front.Y - roofLift*p}
}

// drawWork draws a building's scaffold, its hoist and the loads on it.
func (g *Game) drawWork(screen *ebiten.Image, cam *city.Camera, b *city.Building) {
	up := g.scene.Scaffold(b)
	loads := g.scene.Loads(b.Session.ID)
	if up <= 0 && len(loads) == 0 {
		return
	}
	site, ok := g.workFrame(cam, b)
	if !ok || site.wallTop <= 0 {
		return
	}
	lot, roofLift, storey := site.lot, site.wallTop, site.storey
	lines := roofScaffold(lot, roofLift, storey, up)
	g.drawRoofItems(screen, roofItems(lot, roofLift, up), storey*up)
	// Back to front: the far poles, then the boards, each a tan board with
	// a dark underside, a light top edge and seams, then the near poles
	// and braces, galvanised tubes with a dark edge and a light core.
	// The hoist's line goes on first, so the front corner pole it runs
	// beside is drawn over it in the same silver as the others.
	if up > 0 {
		foot, top := hoistAt(lot, roofLift, 0), hoistAt(lot, roofLift, 1)
		vector.StrokeLine(screen, float32(foot.X), float32(foot.Y), float32(top.X), float32(top.Y-storey), float32(max(1, cam.Zoom)), colorHoist, true)
	}
	for _, s := range scaffoldOrder(lines) {
		if s.plank {
			drawBoard(screen, s)
		} else {
			drawTube(screen, s)
		}
	}
	if up >= 0.5 {
		drawLadder(screen, onRoof(lot, roofLift, 0.5, scaffoldGrid[len(scaffoldGrid)-1]), storey*up*0.5, max(2, roofLift*0.05))
	}
	flash := 0.0
	for _, l := range loads {
		if l.Flash {
			flash = max(flash, 1-l.P)
		}
	}
	for _, at := range workLights(lines) {
		c := colorWorkLight
		c.A = uint8(200 + 55*flash)
		light := float32(max(1.5, roofLift*0.045) * (1 + flash))
		vector.FillCircle(screen, float32(at.X), float32(at.Y), light, c, true)
	}
	if up <= 0 {
		return
	}
	for _, l := range loads {
		if l.Flash {
			continue
		}
		at := hoistAt(lot, roofLift, l.P)
		size := crateSize(roofLift)
		// The last of a burst too close to show one by one stands for
		// the rest, so it is drawn heavier.
		if l.Merged {
			size *= 1.5
		}
		vector.FillRect(screen, float32(at.X-size/2), float32(at.Y-size), float32(size), float32(size), colorLoad, false)
		vector.StrokeRect(screen, float32(at.X-size/2), float32(at.Y-size), float32(size), float32(size), 1, colorHoist, false)
	}
}

var (
	colorSteel = color.NRGBA{0xc0, 0x4a, 0x30, 0xff}
)

// drawRoofItems draws the construction items on a roof deck: a steel
// column as tall as the scaffold, with a crossbar.
func (g *Game) drawRoofItems(screen *ebiten.Image, items []roofItem, storey float64) {
	for _, it := range items {
		x, y, s := float32(it.at.X), float32(it.at.Y), float32(it.size)
		switch it.kind {
		case "column":
			h := float32(storey)
			vector.StrokeLine(screen, x, y, x, y-h, max(2, s*0.35), colorSteel, false)
			vector.StrokeLine(screen, x-s*0.6, y-h*0.6, x+s*0.6, y-h*0.6, max(1.5, s*0.2), colorSteel, false)
		}
	}
}

var (
	colorTubeEdge  = color.NRGBA{0x5c, 0x62, 0x6a, 0xff}
	colorTube      = color.NRGBA{0xc4, 0xca, 0xd2, 0xff}
	colorBoard     = color.NRGBA{0xc8, 0x9a, 0x5c, 0xff}
	colorBoardTop  = color.NRGBA{0xe6, 0xc2, 0x8a, 0xff}
	colorBoardEdge = color.NRGBA{0x6a, 0x48, 0x24, 0xff}
)

// drawTube draws a scaffold tube: a dark edge with a lighter core.
func drawTube(screen *ebiten.Image, s segment) {
	w := float32(max(1, s.width))
	vector.StrokeLine(screen, float32(s.a.X), float32(s.a.Y), float32(s.b.X), float32(s.b.Y), w, colorTubeEdge, true)
	vector.StrokeLine(screen, float32(s.a.X), float32(s.a.Y), float32(s.b.X), float32(s.b.Y), max(0.6, w*0.45), colorTube, true)
}

// drawBoard draws a deck board: a dark underside, the tan board, a light
// top edge, and a dark seam every few pixels along it.
func drawBoard(screen *ebiten.Image, s segment) {
	w := max(1, s.width)
	line := func(dy, width float64, c color.NRGBA) {
		vector.StrokeLine(screen, float32(s.a.X), float32(s.a.Y+dy), float32(s.b.X), float32(s.b.Y+dy), float32(width), c, true)
	}
	line(w*0.45, w*0.5, colorBoardEdge)
	line(0, w, colorBoard)
	line(-w*0.35, w*0.3, colorBoardTop)
	length := math.Hypot(s.b.X-s.a.X, s.b.Y-s.a.Y)
	seams := int(length / max(4, w*2.5))
	for k := 1; k < seams; k++ {
		t := float64(k) / float64(seams)
		x, y := s.a.X+(s.b.X-s.a.X)*t, s.a.Y+(s.b.Y-s.a.Y)*t
		vector.StrokeLine(screen, float32(x), float32(y-w/2), float32(x), float32(y+w/2), 1, colorBoardEdge, true)
	}
}

// drawLadder draws a short ladder standing at foot, height tall: two
// rails and rungs.
func drawLadder(screen *ebiten.Image, foot city.Point, height, width float64) {
	x0, x1 := float32(foot.X-width/2), float32(foot.X+width/2)
	y0, y1 := float32(foot.Y), float32(foot.Y-height)
	vector.StrokeLine(screen, x0, y0, x0, y1, 1, colorTubeEdge, true)
	vector.StrokeLine(screen, x1, y0, x1, y1, 1, colorTubeEdge, true)
	rungs := max(2, int(height/4))
	for k := 1; k < rungs; k++ {
		y := y0 + (y1-y0)*float32(k)/float32(rungs)
		vector.StrokeLine(screen, x0, y, x1, y, 1, colorTubeEdge, true)
	}
}

// The one context reading on the map: four amber lights at the corners
// of a live session's roof once its context is most of the way to a
// compaction, held steady so they never read as needs-you (item 109).
const alertPercent = 80

var colorAlert = color.NRGBA{0xff, 0x9d, 0x1c, 0xff}

// compactionAlert is whether a building's roof lights are lit.
func compactionAlert(b *city.Building) bool {
	return state.Live(b.Session.State) && b.Session.ContextPercent >= alertPercent
}

// alertLights is where the lights stand: the corners of a roof wallTop
// above the footprint lot.
func alertLights(lot [4]city.Point, wallTop float64) []city.Point {
	out := make([]city.Point, 0, len(lot))
	for _, c := range lot {
		out = append(out, city.Point{X: c.X, Y: c.Y - wallTop})
	}
	return out
}

// alertColor is the lights' colour at a moment, the same at every one.
func alertColor(float64) color.NRGBA { return colorAlert }

// drawAlert lights a building's roof corners while its session nears a
// compaction.
func (g *Game) drawAlert(screen *ebiten.Image, cam *city.Camera, b *city.Building, seconds float64) {
	if !compactionAlert(b) {
		return
	}
	site, ok := g.workFrame(cam, b)
	if !ok {
		return
	}
	r := float32(max(1.5, site.storey*0.07))
	c := alertColor(seconds)
	glow := c
	glow.A = 0x55
	for _, at := range alertLights(site.lot, site.wallTop) {
		vector.FillCircle(screen, float32(at.X), float32(at.Y), r*2.4, glow, true)
		vector.FillCircle(screen, float32(at.X), float32(at.Y), r, c, true)
	}
}
