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
// windows, at the building's context fill. Each tool call sends a pulse
// up the face toward the camera from that floor to the roof, and at still
// it is a brief brightening of the whole face instead.

var (
	colorWorkingFloor = color.NRGBA{0xff, 0xe0, 0x8a, 0xd0}
	colorPulse        = color.NRGBA{0xff, 0xf4, 0xc8, 0xff}
)

// workingFloorBand is the working floor's band across a building drawn
// at r, floor of the way up from its foot.
func workingFloorBand(r city.Rect, floor float64) city.Rect {
	h := r.Height()
	y := r.Max.Y - h*0.1 - floor*h*0.8
	band := h * 0.06
	return city.Rect{
		Min: city.Point{X: r.Min.X + r.Width()*0.15, Y: y - band/2},
		Max: city.Point{X: r.Max.X - r.Width()*0.15, Y: y + band/2},
	}
}

// pulseAt is where a pulse is p of the way up a building drawn at r:
// from the working floor at the start to the roof at the end.
func pulseAt(r city.Rect, floor, p float64) city.Point {
	from := workingFloorBand(r, floor).Center().Y
	return city.Point{X: r.Min.X + r.Width()/2, Y: from + (r.Min.Y-from)*p}
}

// drawWork lights a mid-turn building's working floor and runs its
// pulses, on the building's own drawn rect r.
func (g *Game) drawWork(screen *ebiten.Image, b *city.Building, r city.Rect) {
	if r.Area() == 0 {
		return
	}
	floor, working := g.scene.WorkingFloor(b)
	if working {
		band := workingFloorBand(r, floor)
		vector.FillRect(screen, float32(band.Min.X), float32(band.Min.Y), float32(band.Width()), float32(band.Height()), colorWorkingFloor, false)
	}
	for _, p := range g.scene.Pulses(b.Session.ID) {
		if p.Brighten {
			glow := colorPulse
			glow.A = uint8(90 * (1 - p.P))
			vector.FillRect(screen, float32(r.Min.X), float32(r.Min.Y), float32(r.Width()), float32(r.Height()), glow, false)
			continue
		}
		at := pulseAt(r, floor, p.P)
		radius := float32(max(2, r.Width()*0.05))
		c := colorPulse
		c.A = uint8(255 * (1 - p.P*0.6))
		vector.FillCircle(screen, float32(at.X), float32(at.Y), radius, c, true)
	}
}
