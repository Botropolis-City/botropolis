package render

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
)

// Drones in flight, with their shadows. Item 89.

const (
	// droneCruise is how far above the roofline a drone loiters, in
	// pixels at zoom 1.
	droneCruise = droneHover
	// loiterRadius is the loiter's mean radius in world units, and
	// loiterPeriod how long one loop takes at full pace.
	loiterRadius = city.Tile * 1.2
	loiterPeriod = 8.0
)

var colorDroneShadow = color.NRGBA{0, 0, 0, 0x50}

// droneAt is where a drone is drawn on screen and the world point its
// shadow falls on, over a roof roofLift pixels above the ground. The
// loiter is a loose, uneven circle that widens as the drone climbs, and
// each seat flies its own.
func droneAt(cam *city.Camera, f city.Flight, roofLift, seconds float64) (city.Point, city.Point) {
	seat := float64(f.Seat)
	speed := 1 + 0.17*seat
	theta := 2*math.Pi*seconds*speed/loiterPeriod + seat*2*math.Pi/3
	r := loiterRadius * f.Climb * (1 + 0.22*math.Sin(2*theta+seat) + 0.08*math.Sin(3*theta+1.3*seat))
	shadow := city.Point{X: f.Over.X + r*math.Cos(theta), Y: f.Over.Y + r*math.Sin(theta)}
	bob := 2 * math.Sin(seconds*3+seat) * f.Climb
	ground := cam.WorldToScreen(shadow)
	lift := roofLift + (droneCruise+bob)*cam.Zoom*f.Climb
	return city.Point{X: ground.X, Y: ground.Y - lift}, shadow
}

// layered is the frame's paint order: everything on the ground in the
// order it stands, then everything in the air, so nothing flying is ever
// drawn behind a building it is above.
func layered(cam *city.Camera, standing []drawable, airborne []func()) []func() {
	var out []func()
	for _, item := range order(cam, standing) {
		out = append(out, item.draw)
	}
	return append(out, airborne...)
}

// flights is every drone the map draws: the world points their shadows
// fall on, and the drones themselves for the airborne pass.
func (g *Game) flights(screen *ebiten.Image, c *city.City, cam *city.Camera, seconds float64) (shadows []city.Point, airborne []func()) {
	if !g.shows(city.NetworkCranes) {
		return nil, nil
	}
	for _, b := range c.Buildings() {
		if b.Vacant || g.scene.Dimmed(b) {
			continue
		}
		b := b
		lift := g.kitRoofLift(cam, buildingPiece(b))
		for _, f := range g.scene.Flights(b.Session.ID) {
			at, shadow := droneAt(cam, f, lift, seconds)
			shadows = append(shadows, shadow)
			fade := f.Fade
			airborne = append(airborne, func() {
				var tint *ebiten.ColorScale
				if fade < 1 {
					tint = &ebiten.ColorScale{}
					tint.ScaleAlpha(float32(fade))
				}
				drone := g.kitAt(screen, cam, kitDrone, 0, at, tint)
				g.noteHit(drone, city.Hit{Building: b, District: g.scene.City().DistrictOf(b), Subagent: b})
			})
		}
	}
	return shadows, airborne
}

// droneShadow is a soft ellipse on the ground beneath a drone.
func droneShadow(screen *ebiten.Image, cam *city.Camera, at city.Point) {
	p := cam.WorldToScreen(at)
	rx, ry := 5*cam.Zoom, 2.5*cam.Zoom
	var path vector.Path
	const steps = 16
	for i := 0; i < steps; i++ {
		a := 2 * math.Pi * float64(i) / steps
		x, y := float32(p.X+rx*math.Cos(a)), float32(p.Y+ry*math.Sin(a))
		if i == 0 {
			path.MoveTo(x, y)
		} else {
			path.LineTo(x, y)
		}
	}
	path.Close()
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(colorDroneShadow)
	vector.FillPath(screen, &path, nil, op)
}
