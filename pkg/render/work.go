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

// roofScaffold is the scaffold on a roof roofLift above a footprint lot
// on screen, up of the way raised: a stout pole at each corner, a little
// in, and two levels of planks round them, halfway and at the top.
func roofScaffold(lot [4]city.Point, roofLift, up float64) []segment {
	if up <= 0 {
		return nil
	}
	centre := city.Point{}
	for _, c := range lot {
		centre.X, centre.Y = centre.X+c.X/4, centre.Y+c.Y/4
	}
	h := storeyOf(roofLift) * up
	var poles, tops []city.Point
	for _, c := range lot {
		foot := lerpPoint(c, centre, 0.12)
		foot.Y -= roofLift
		poles = append(poles, foot)
		tops = append(tops, city.Point{X: foot.X, Y: foot.Y - h})
	}
	pole := max(2, roofLift*0.035)
	plank := max(2, roofLift*0.05)
	var out []segment
	for i := range poles {
		out = append(out, segment{a: poles[i], b: tops[i], width: pole, pole: true})
	}
	for _, level := range []float64{0.5, 1} {
		for i := range poles {
			j := (i + 1) % len(poles)
			at := func(k int) city.Point { return city.Point{X: poles[k].X, Y: poles[k].Y - h*level} }
			out = append(out, segment{a: at(i), b: at(j), width: plank, plank: true})
		}
	}
	return out
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
