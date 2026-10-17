package render

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
)

// Drones in flight, with their shadows. Item 89.
//
// A drone works on a building: it flits between points on the two faces
// toward the camera and over the roof, pausing at each, the way someone
// inspecting it would. Its points come from its seat and its rhythm is
// its own, so several on one building never move in step. Between
// districts it flies above the rooftops.

const (
	// droneCruise is how far above the roofline a drone flies between
	// districts, in pixels at zoom 1.
	droneCruise = droneHover
	// droneWindow is how wide a drone is drawn against its building's
	// drawn width: about one window.
	droneWindow = 0.15
	// droneHop is how far above the roof a drone works there, in pixels
	// at zoom 1, and droneOff how far off a face, in world units.
	droneHop = 6.0
	droneOff = 1.0
	// droneDwell is the share of each beat a drone holds still: short,
	// so it reads as busy.
	droneDwell = 0.3
)

var colorDroneShadow = color.NRGBA{0, 0, 0, 0x50}

// station is one point a drone works at: on the faces toward the camera,
// s from 0 to 2 along them and v up them from 0 to 1, or over the roof
// at a world point.
type station struct {
	roof bool
	s, v float64
	at   city.Point
}

// spot is a drone's ground point and how high above it it is drawn.
type spot struct {
	base city.Point
	lift float64
}

// droneAt is where a drone is drawn on screen and the world point its
// shadow falls on. roofLift is how far its building's roof stands above
// the ground on screen.
func droneAt(cam *city.Camera, f city.Flight, roofLift, seconds float64) (city.Point, city.Point) {
	var at spot
	if f.On == nil {
		at = spot{base: f.Over, lift: roofLift + droneCruise*cam.Zoom}
	} else {
		r := f.On.Rect
		work := droneWork(cam, r, roofLift, f.Seat, seconds)
		pad := spot{base: r.Center(), lift: roofLift}
		at = spot{base: lerpPoint(pad.base, work.base, f.Climb), lift: pad.lift + (work.lift-pad.lift)*f.Climb}
	}
	ground := cam.WorldToScreen(at.base)
	return city.Point{X: ground.X, Y: ground.Y - at.lift}, at.base
}

// droneWork is where a working drone is at seconds: held at a station
// for most of each beat, then moving to the next.
func droneWork(cam *city.Camera, r city.Rect, roofLift float64, seat int, seconds float64) spot {
	period := 0.55 + 0.13*float64(seat)
	beat := seconds/period + 0.73*float64(seat)
	n := math.Floor(beat)
	from := droneStation(r, seat, int(n))
	frac := beat - n
	if frac < droneDwell {
		return stationSpot(cam, r, roofLift, from)
	}
	to := droneStation(r, seat, int(n)+1)
	p := (frac - droneDwell) / (1 - droneDwell)
	p = p * p * (3 - 2*p)
	return moveSpot(cam, r, roofLift, from, to, p)
}

// droneStation is a seat's nth station, the same every time it is asked.
func droneStation(r city.Rect, seat, n int) station {
	h := func(k int) float64 {
		x := math.Sin(float64(seat)*12.9898+float64(n)*78.233+float64(k)*37.719) * 43758.5453
		return x - math.Floor(x)
	}
	if h(0) > 0.7 {
		return station{roof: true, at: city.Point{
			X: r.Min.X + r.Width()*(0.25+0.5*h(1)),
			Y: r.Min.Y + r.Height()*(0.25+0.5*h(2)),
		}}
	}
	s := 0.15 + 0.7*h(1)
	if h(0) > 0.35 {
		s++
	}
	return station{s: s, v: 0.25 + 0.65*h(2)}
}

// stationSpot is a station on screen. A roof station is drawn above the
// roof plane; a face station just off the face.
func stationSpot(cam *city.Camera, r city.Rect, roofLift float64, st station) spot {
	if st.roof {
		return spot{base: st.at, lift: roofLift + droneHop*cam.Zoom}
	}
	return spot{base: faceBase(cam, r, st.s), lift: st.v * roofLift}
}

// moveSpot is a drone p of the way from one station to the next. Along
// the faces it follows them; to the roof it climbs its face first and
// then crosses, and from the roof it does the reverse, so it never
// passes through the building.
func moveSpot(cam *city.Camera, r city.Rect, roofLift float64, from, to station, p float64) spot {
	top := roofLift + droneHop*cam.Zoom
	switch {
	case !from.roof && !to.roof:
		return spot{base: faceBase(cam, r, from.s+(to.s-from.s)*p), lift: (from.v + (to.v-from.v)*p) * roofLift}
	case from.roof && to.roof:
		return spot{base: lerpPoint(from.at, to.at, p), lift: top}
	case to.roof:
		edge := faceBase(cam, r, from.s)
		if p < 0.5 {
			return spot{base: edge, lift: from.v*roofLift + (top-from.v*roofLift)*p*2}
		}
		return spot{base: lerpPoint(edge, to.at, (p-0.5)*2), lift: top}
	}
	edge := faceBase(cam, r, to.s)
	if p < 0.5 {
		return spot{base: lerpPoint(from.at, edge, p*2), lift: top}
	}
	return spot{base: edge, lift: top + (to.v*roofLift-top)*(p-0.5)*2}
}

// faceBase is the ground point s along the faces toward the camera, from
// one side corner through the front corner to the other, just off them.
func faceBase(cam *city.Camera, r city.Rect, s float64) city.Point {
	front, sides := frontCorner(cam, r)
	from, to, t := sides[0], front, s
	if s > 1 {
		from, to, t = front, sides[1], s-1
	}
	at := lerpPoint(from, to, t)
	edge := city.Point{X: to.X - from.X, Y: to.Y - from.Y}
	length := math.Hypot(edge.X, edge.Y)
	if length == 0 {
		return at
	}
	normal := city.Point{X: -edge.Y / length, Y: edge.X / length}
	centre := r.Center()
	if normal.X*(at.X-centre.X)+normal.Y*(at.Y-centre.Y) < 0 {
		normal = city.Point{X: -normal.X, Y: -normal.Y}
	}
	return city.Point{X: at.X + normal.X*droneOff, Y: at.Y + normal.Y*droneOff}
}

// frontCorner is the footprint's corner nearest the camera and the two
// corners beside it, whose edges are the faces the camera sees.
func frontCorner(cam *city.Camera, r city.Rect) (city.Point, [2]city.Point) {
	corners := [4]city.Point{r.Min, {X: r.Max.X, Y: r.Min.Y}, r.Max, {X: r.Min.X, Y: r.Max.Y}}
	best, depth := 0, math.Inf(-1)
	for i, c := range corners {
		t := cam.TurnPoint(c)
		if d := t.X + t.Y; d > depth {
			best, depth = i, d
		}
	}
	return corners[best], [2]city.Point{corners[(best+3)%4], corners[(best+1)%4]}
}

func lerpPoint(a, b city.Point, p float64) city.Point {
	return city.Point{X: a.X + (b.X-a.X)*p, Y: a.Y + (b.Y-a.Y)*p}
}

// segmentDistance is how far p is from the segment a–b.
func segmentDistance(p, a, b city.Point) float64 {
	d := city.Point{X: b.X - a.X, Y: b.Y - a.Y}
	l := d.X*d.X + d.Y*d.Y
	t := 0.0
	if l > 0 {
		t = math.Max(0, math.Min(1, ((p.X-a.X)*d.X+(p.Y-a.Y)*d.Y)/l))
	}
	return math.Hypot(p.X-(a.X+d.X*t), p.Y-(a.Y+d.Y*t))
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

// droneAtDepth is a drone in the depth sort, standing on the ground point
// its shadow falls on, as a car stands on its road.
func droneAtDepth(cam *city.Camera, shadow city.Point, draw func()) drawable {
	return footprintAt(cam, city.RectAt(shadow.X-0.5, shadow.Y-0.5, 1, 1), draw)
}

// flights is every drone the map draws: the world points their shadows
// fall on, and the drones themselves for the depth sort.
func (g *Game) flights(screen *ebiten.Image, c *city.City, cam *city.Camera, seconds float64) (shadows []city.Point, drones []drawable) {
	if !g.shows(city.NetworkCranes) {
		return nil, nil
	}
	for _, b := range c.Buildings() {
		if b.Vacant || g.scene.Dimmed(b) {
			continue
		}
		b := b
		for _, f := range g.scene.Flights(b.Session.ID) {
			on := b
			if f.On != nil {
				on = f.On
			}
			lift := g.kitRoofLift(cam, buildingPiece(on))
			at, shadow := droneAt(cam, f, lift, seconds)
			shadows = append(shadows, shadow)
			fade := f.Fade
			shrink := g.droneShrink(cam, buildingPiece(on))
			drones = append(drones, droneAtDepth(cam, shadow, func() {
				var tint *ebiten.ColorScale
				if fade < 1 {
					tint = &ebiten.ColorScale{}
					tint.ScaleAlpha(float32(fade))
				}
				// A dark outline, so a drone reads on any ground.
				for _, d := range []city.Point{{X: -1, Y: 0}, {X: 1, Y: 0}, {X: 0, Y: -1}, {X: 0, Y: 1}} {
					dark := &ebiten.ColorScale{}
					dark.Scale(0, 0, 0, float32(fade)*0.9)
					g.kitAtShrunk(screen, cam, kitDrone, city.Point{X: at.X + d.X, Y: at.Y + d.Y}, shrink, dark)
				}
				drone := g.kitAtShrunk(screen, cam, kitDrone, at, shrink, tint)
				g.noteHit(drone, city.Hit{Building: b, District: g.scene.City().DistrictOf(b), Subagent: b})
			}))
		}
	}
	return shadows, drones
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

// droneMinWidth is the narrowest a drone is drawn, in pixels, so one
// reads even at fit.
const droneMinWidth = 12.0

// droneWidth is how wide a drone is drawn on a building drawn
// buildingWidth pixels wide: one window, and never under droneMinWidth.
func droneWidth(buildingWidth float64) float64 {
	return max(droneWindow*buildingWidth, droneMinWidth)
}

// droneShrink is the shrink that draws a drone droneWidth wide on a
// building drawn as piece.
func (g *Game) droneShrink(cam *city.Camera, piece string) float64 {
	drone, building := g.kitSize(cam, kitDrone), g.kitSize(cam, piece)
	if drone.X == 0 {
		return droneWindow
	}
	return droneWidth(float64(building.X)) / float64(drone.X)
}

// kitAtShrunk is kitAt at a fraction of the piece's size.
func (g *Game) kitAtShrunk(screen *ebiten.Image, cam *city.Camera, name string, foot city.Point, shrink float64, tint *ebiten.ColorScale) city.Rect {
	if g.kits == nil {
		return city.Rect{}
	}
	atlas := g.kits.pick(cam.Zoom)
	if atlas == nil {
		return city.Rect{}
	}
	sprite, ok := atlas.Sprite(name, cam.Heading)
	if !ok || sprite.Page >= len(atlas.pages) {
		return city.Rect{}
	}
	scale := cam.Zoom / atlas.Zoom * shrink
	origin := city.Point{X: foot.X - float64(sprite.Anchor.X)*scale, Y: foot.Y - float64(sprite.Anchor.Y)*scale}
	img := atlas.pages[sprite.Page].SubImage(sprite.Rect).(*ebiten.Image)
	g.drawSprite(screen, img, origin, scale, tint)
	return city.RectAt(origin.X, origin.Y, float64(sprite.Rect.Dx())*scale, float64(sprite.Rect.Dy())*scale)
}
