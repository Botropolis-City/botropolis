package render

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
)

// The building's own work. Item 89.
//
// While a session is mid-turn its roof is scaffolded with work lights,
// in slice 2's construction strokes, and each tool call runs a load up a
// hoist on the building's outside front corner to the scaffold. When the
// turn ends the scaffold comes down. At still a load is a flash of the
// work lights.

var (
	colorWorkLight = color.NRGBA{0xff, 0xd8, 0x60, 0xff}
	colorLoad      = color.NRGBA{0xc8, 0x8a, 0x3c, 0xff}
	colorHoist     = color.NRGBA{0x3a, 0x34, 0x2c, 0xff}
	colorPlank     = color.NRGBA{0x9a, 0x74, 0x48, 0xff}
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

// storeyOf is how high the roof scaffold stands: a storey of a building
// whose roof is roofLift up.
func storeyOf(roofLift float64) float64 { return roofLift * 0.3 }

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
func roofScaffold(lot [4]city.Point, roofLift, up float64) []segment {
	if up <= 0 {
		return nil
	}
	h := storeyOf(roofLift) * up
	pole := max(2, roofLift*0.035)
	plank := max(2, roofLift*0.05)
	var out []segment
	for _, u := range scaffoldGrid {
		for _, v := range scaffoldGrid {
			foot := onRoof(lot, roofLift, u, v)
			out = append(out, segment{a: foot, b: city.Point{X: foot.X, Y: foot.Y - h}, width: pole, pole: true})
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

// roofItem is a construction item on a scaffolded roof deck: a pallet
// the hoist's loads land on, a stack of materials, or a column of the
// next storey's frame rising.
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
	return []roofItem{
		{kind: "pallet", at: onRoof(lot, roofLift, 0.78, 0.78), size: size},
		{kind: "materials", at: onRoof(lot, roofLift, 0.3, 0.68), size: size},
		{kind: "column", at: onRoof(lot, roofLift, 0.5, 0.3), size: size},
	}
}

// crateSize is how big a load's crate is drawn on a building whose roof
// is roofLift up: big enough to read at the opening zoom.
func crateSize(roofLift float64) float64 { return max(6, roofLift*0.12) }

// hoistAt is where a load is p of the way up the hoist on the outside
// front corner: at the foot at the start, at the roof's scaffold at the
// end.
func hoistAt(lot [4]city.Point, roofLift, p float64) city.Point {
	front, _ := frontOf(lot)
	return city.Point{X: front.X, Y: front.Y - roofLift*p}
}

// drawWork draws a building's scaffold, its hoist and the loads on it.
func (g *Game) drawWork(screen *ebiten.Image, cam *city.Camera, b *city.Building) {
	up := g.scene.Scaffold(b)
	loads := g.scene.Loads(b.Session.ID)
	if up <= 0 && len(loads) == 0 {
		return
	}
	lot := lotCorners(cam, b.Rect)
	roofLift := g.kitRoofLift(cam, buildingPiece(b))
	if roofLift <= 0 {
		return
	}
	lines := roofScaffold(lot, roofLift, up)
	for _, s := range lines {
		c := colorScaffold
		if s.plank {
			c = colorPlank
		}
		vector.StrokeLine(screen, float32(s.a.X), float32(s.a.Y), float32(s.b.X), float32(s.b.Y), float32(max(1, s.width)), c, true)
	}
	g.drawRoofItems(screen, roofItems(lot, roofLift, up), storeyOf(roofLift)*up)
	flash := 0.0
	for _, l := range loads {
		if l.Flash {
			flash = max(flash, 1-l.P)
		}
	}
	// The work lights stand on every pole but the back one, the highest
	// on screen, which the building hides.
	back := -1
	for i, s := range lines {
		if s.pole && (back < 0 || s.a.Y < lines[back].a.Y) {
			back = i
		}
	}
	for i, s := range lines {
		if !s.pole || i == back {
			continue
		}
		c := colorWorkLight
		c.A = uint8(200 + 55*flash)
		light := float32(max(1.5, roofLift*0.04) * (1 + flash))
		vector.FillCircle(screen, float32(s.b.X), float32(s.b.Y), light, c, true)
	}
	if up <= 0 {
		return
	}
	foot, top := hoistAt(lot, roofLift, 0), hoistAt(lot, roofLift, 1)
	vector.StrokeLine(screen, float32(foot.X), float32(foot.Y), float32(top.X), float32(top.Y-storeyOf(roofLift)), float32(max(1, cam.Zoom)), colorHoist, true)
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
	colorPallet    = color.NRGBA{0x8a, 0x64, 0x3c, 0xff}
	colorMaterials = color.NRGBA{0x9a, 0x9c, 0xa4, 0xff}
	colorSteel     = color.NRGBA{0xc0, 0x4a, 0x30, 0xff}
)

// drawRoofItems draws the construction items on a roof deck: a flat
// pallet, a stack of two blocks, and a steel column as tall as the
// scaffold, with a crossbar.
func (g *Game) drawRoofItems(screen *ebiten.Image, items []roofItem, storey float64) {
	for _, it := range items {
		x, y, s := float32(it.at.X), float32(it.at.Y), float32(it.size)
		switch it.kind {
		case "pallet":
			vector.FillRect(screen, x-s, y-s*0.3, 2*s, s*0.3, colorPallet, false)
			vector.StrokeRect(screen, x-s, y-s*0.3, 2*s, s*0.3, 1, colorHoist, false)
		case "materials":
			for k := float32(0); k < 2; k++ {
				top := y - s*0.6*(k+1)
				vector.FillRect(screen, x-s*0.7, top, s*1.4, s*0.6, colorMaterials, false)
				vector.StrokeRect(screen, x-s*0.7, top, s*1.4, s*0.6, 1, colorHoist, false)
			}
		case "column":
			h := float32(storey)
			vector.StrokeLine(screen, x, y, x, y-h, max(2, s*0.35), colorSteel, false)
			vector.StrokeLine(screen, x-s*0.6, y-h*0.6, x+s*0.6, y-h*0.6, max(1.5, s*0.2), colorSteel, false)
		}
	}
}
