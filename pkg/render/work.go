package render

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
)

// The building's own work. Item 89.
//
// While a session is mid-turn one floor glows brighter than its lit
// windows, at the building's context fill: a band round the two faces
// toward the camera. Each tool call sends a pulse up the front corner
// from that floor to the roof, and at still it is a brief brightening of
// the faces instead.

var (
	colorWorkingFloor = color.NRGBA{0xff, 0xe0, 0x8a, 0xc0}
	colorPulse        = color.NRGBA{0xff, 0xf4, 0xc8, 0xff}
)

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

// workingFloor is the working floor's band on each face toward the
// camera, floor of the way up a building whose roof stands roofLift
// above its footprint lot on screen.
func workingFloor(lot [4]city.Point, roofLift, floor float64) [2][4]city.Point {
	front, sides := frontOf(lot)
	band := max(2, roofLift*0.06)
	lo, hi := floor*roofLift-band/2, floor*roofLift+band/2
	up := func(p city.Point, h float64) city.Point { return city.Point{X: p.X, Y: p.Y - h} }
	var out [2][4]city.Point
	for i, side := range sides {
		out[i] = [4]city.Point{up(front, lo), up(side, lo), up(side, hi), up(front, hi)}
	}
	return out
}

// pulseAt is where a pulse is p of the way up the front corner: from the
// working floor at the start to the roof at the end.
func pulseAt(lot [4]city.Point, roofLift, floor, p float64) city.Point {
	front, _ := frontOf(lot)
	h := floor*roofLift + (roofLift-floor*roofLift)*p
	return city.Point{X: front.X, Y: front.Y - h}
}

// drawWork lights a mid-turn building's working floor and runs its
// pulses.
func (g *Game) drawWork(screen *ebiten.Image, cam *city.Camera, b *city.Building) {
	lot := lotCorners(cam, b.Rect)
	roofLift := g.kitRoofLift(cam, buildingPiece(b))
	if roofLift <= 0 {
		return
	}
	floor, working := g.scene.WorkingFloor(b)
	if working {
		for _, face := range workingFloor(lot, roofLift, floor) {
			fillQuad(screen, face, colorWorkingFloor)
		}
	}
	for _, p := range g.scene.Pulses(b.Session.ID) {
		if p.Brighten {
			glow := colorPulse
			glow.A = uint8(90 * (1 - p.P))
			for _, face := range workingFloor(lot, roofLift, 0.5) {
				fillQuad(screen, [4]city.Point{face[0], face[1], {X: face[1].X, Y: face[1].Y - roofLift/2}, {X: face[0].X, Y: face[0].Y - roofLift/2}}, glow)
			}
			continue
		}
		at := pulseAt(lot, roofLift, floor, p.P)
		radius := float32(max(2, roofLift*0.05))
		c := colorPulse
		c.A = uint8(255 * (1 - p.P*0.6))
		// The last of a burst too close to show one by one stands for
		// the rest, so it is drawn bigger and at full brightness.
		if p.Merged {
			radius *= 1.6
			c.A = 255
		}
		vector.FillCircle(screen, float32(at.X), float32(at.Y), radius, c, true)
	}
}
