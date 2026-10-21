package render

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/shapes"
	"botropolis.city/botropolis/pkg/ui"
)

// Bug 45. A project's name stands at the near corner of its lot as a
// monument sign, built to the grammar of the references in
// docs/references: coursed piers either side of a recessed panel, a
// cornice oversailing them, a base course at the ground.
//
// It faces the viewer. Type sheared into a 2:1 plane is hard to read at
// any size, so making the sign bigger would only have produced bigger
// slanted type; turning the face square to the screen is what fixes it.
//
// Facing the viewer is also what got the item 4 billboards and the item
// 40 plate rejected, so the difference has to be earned rather than
// assumed. A decal is what was rejected, not a head-on object. Three
// things keep this a solid: the piers and cornice keep visible returns,
// the cornice keeps a darker underside, and the sign casts a contact
// shadow on the ground. It is a drawable on its own footprint, so it
// occludes and is occluded like anything else — it is not painted last,
// which is the thing that made the plate chrome.
const (
	// monumentSpan is how wide the sign is against its planting bed.
	monumentSpan = 1.9
	// monumentDepth is how far the returns step back, as a share of the
	// sign's width. A head-on object with no visible depth is a sticker.
	monumentDepth = 0.05
	// bedForward is how far in front of the sign its planting sits, and
	// bedAside how far to one side. It goes beside the base rather than
	// dead in front of it: the references plant *around* a monument,
	// and a bed centred on the panel just masks the name.
	bedForward = 0.22
	bedAside   = 0.62
	// kerbClear is how much clear ground is left between the sign's own
	// footprint and the lot's edge, on top of the footprint itself.
	kerbClear = city.Tile * 0.6
)

// The masonry, in the references' palette: pale coursed stone, a dark
// bronze panel, pale copy on it. Two materials, not one.
var (
	colorMonumentStone   = color.NRGBA{0xcf, 0xc9, 0xbb, 0xff}
	colorMonumentCornice = color.NRGBA{0xdd, 0xd8, 0xcb, 0xff}
	colorMonumentReturn  = color.NRGBA{0x9d, 0x97, 0x8a, 0xff}
	colorMonumentSoffit  = color.NRGBA{0x77, 0x72, 0x68, 0xff}
	colorMonumentCourse  = color.NRGBA{0xb3, 0xac, 0x9d, 0xff}
	colorMonumentPanel   = color.NRGBA{0x4a, 0x3b, 0x31, 0xff}
	colorMonumentReveal  = color.NRGBA{0x2c, 0x22, 0x1c, 0xff}
	colorMonumentCopy    = color.NRGBA{0xf0, 0xe9, 0xdc, 0xff}
	colorMonumentShadow  = color.NRGBA{0x18, 0x18, 0x1c, 0x66}
	colorMonumentLamp    = color.NRGBA{0xf0, 0xd8, 0x9c, 0x38}
)

// screenStep is the world vector a screen-space offset stands for.
// Unproject is linear, so the step does not depend on where it is
// measured from.
func screenStep(cam *city.Camera, d city.Point) city.Point {
	return cam.Unproject(city.Point{X: d.X / cam.Zoom, Y: d.Y / cam.Zoom})
}

// bedSteps are the screen-space offsets of the planting bed's two ends
// from the sign's ground point, and signSteps the sign's own two ends.
// Between them they are every piece of ground the monument covers.
func monumentSteps(width, bedWidth float64, zoom float64) [4]city.Point {
	forward := bedForward * city.Tile * zoom
	aside := width * bedAside / 2
	return [4]city.Point{
		{X: -width / 2},
		{X: width / 2},
		{X: -aside - bedWidth/2, Y: forward},
		{X: -aside + bedWidth/2, Y: forward},
	}
}

// monumentGround is the world box a monument covers on the ground,
// relative to its own ground point.
//
// The sign faces the viewer, so its width runs across the *screen* and
// the ground it covers is a diagonal in the world that turns with the
// heading. A margin chosen by eye cannot know that, and cannot know
// about the base's oversail or the planting bed either — which is why
// the first inset left the base sitting out over the kerb. This is the
// third time on this project that a guard has been put on an anchor
// while the thing that misbehaved was a wide object around it: bug 39's
// reservation that was not the building, and bug 41's cell that was not
// the tree.
func monumentGround(cam *city.Camera, width, bedWidth float64) city.Rect {
	box := city.Rect{
		Min: city.Point{X: math.Inf(1), Y: math.Inf(1)},
		Max: city.Point{X: math.Inf(-1), Y: math.Inf(-1)},
	}
	for _, step := range monumentSteps(width, bedWidth, cam.Zoom) {
		w := screenStep(cam, step)
		box.Min.X = math.Min(box.Min.X, w.X)
		box.Min.Y = math.Min(box.Min.Y, w.Y)
		box.Max.X = math.Max(box.Max.X, w.X)
		box.Max.Y = math.Max(box.Max.Y, w.Y)
	}
	return box
}

// monumentSite is where a project's sign stands: the corner of its lot
// nearest the viewer, pulled in far enough that the whole of the sign —
// base course, oversail and planting — stands on the lot's own ground
// rather than out on the kerb.
//
// The pull-back is derived from the sign's own footprint rather than
// chosen, and it is asymmetric, because the ground a screen-facing
// object covers is not centred on its anchor.
//
// The corner *is* recomputed as the camera turns, and that is the
// opposite of the chimney, which must not be. A chimney is a fixture of
// its building and keeps its place when you walk round it. A sign is
// sited to be read. Aria ruled for the near corner on the r222 frames.
//
// It reports false when the lot is too small to hold the sign at this
// heading, which is a fact about the lot rather than about the sign.
func monumentSite(cam *city.Camera, lot city.Rect, width, bedWidth float64) (city.Point, bool) {
	ground := monumentGround(cam, width, bedWidth)
	room := city.Rect{
		Min: city.Point{X: lot.Min.X - ground.Min.X + kerbClear, Y: lot.Min.Y - ground.Min.Y + kerbClear},
		Max: city.Point{X: lot.Max.X - ground.Max.X - kerbClear, Y: lot.Max.Y - ground.Max.Y - kerbClear},
	}
	if room.Min.X > room.Max.X || room.Min.Y > room.Max.Y {
		return city.Point{}, false
	}
	best := room.Min
	for _, p := range [3]city.Point{
		{X: room.Max.X, Y: room.Min.Y}, room.Max, {X: room.Min.X, Y: room.Max.Y},
	} {
		if cam.Depth(p) > cam.Depth(best) {
			best = p
		}
	}
	return best, true
}

// bedOf is where the sign's planting sits: beside its foot, a little in
// front. Screen space throughout — the first cut mixed screen pixels
// into a map-plane coordinate, so the bed drifted as the zoom changed.
func bedOf(cam *city.Camera, at city.Point, width float64) city.Point {
	on := cam.WorldToScreen(at)
	on.X -= width * bedAside / 2
	on.Y += bedForward * city.Tile * cam.Zoom
	return cam.ScreenToWorld(on)
}

// ellipse fills a flattened disc, for the contact patch a sign makes on
// the ground it stands on.
func (g *Game) ellipse(screen *ebiten.Image, r city.Rect, c color.NRGBA) {
	cx, cy := r.Center().X, r.Center().Y
	rx, ry := r.Width()/2, r.Height()/2
	pts := shapes.AppendEllipse(g.paint.Points(), city.Point{X: cx, Y: cy}, rx, ry)
	g.paint.Fill(screen, pts, c)
	g.paint.Keep(pts)
}

// returnOf draws the side of a block stepping back from its right edge,
// so a face-on element still shows its depth.
func (g *Game) returnOf(screen *ebiten.Image, r city.Rect, depth float64, c color.NRGBA) {
	side := [4]city.Point{{X: r.Max.X, Y: r.Min.Y}, {X: r.Max.X + depth, Y: r.Min.Y - depth/2}, {X: r.Max.X + depth, Y: r.Max.Y - depth/2}, {X: r.Max.X, Y: r.Max.Y}}
	g.paint.Fill(screen, side[:], c)
}

func (g *Game) fill(screen *ebiten.Image, r city.Rect, c color.NRGBA) {
	vector.FillRect(screen, float32(r.Min.X), float32(r.Min.Y), float32(r.Width()), float32(r.Height()), c, false)
}

// monumentOf is where a district's sign stands and how big it is, or
// false when the lot has nowhere to put it.
func (g *Game) monumentOf(cam *city.Camera, d *city.District) (at city.Point, width, bed float64, ok bool) {
	size := g.kitSize(cam, kitPlanter)
	if size.X == 0 {
		return city.Point{}, 0, 0, false
	}
	bed = float64(size.X)
	width = bed * monumentSpan
	at, ok = monumentSite(cam, d.Rect, ui.MonumentFootprint(width), bed)
	return at, width, bed, ok
}

func (g *Game) districtMonument(screen *ebiten.Image, cam *city.Camera, d *city.District, night bool) {
	at, width, bed, ok := g.monumentOf(cam, d)
	if !ok {
		return
	}
	ground := cam.WorldToScreen(at)
	sign, ok := ui.LayoutMonument(d.Name, ground, width, g.faces.Measure)
	if !ok {
		g.kit(screen, cam, kitPlanter, 0, bedOf(cam, at, bed), nil)
		return
	}
	depth := sign.Whole().Width() * monumentDepth

	// The ground first: the contact patch is what stops a head-on
	// object hovering, and it is the thing the Oak Hollow reference has.
	g.ellipse(screen, sign.Shadow, colorMonumentShadow)
	if night {
		lamp := city.Point{X: ground.X, Y: ground.Y - sign.Base.Height()*0.4}
		g.glow(screen, lamp, lit(sign.Base.Width()*0.4, minMonumentGlow), boost(colorMonumentLamp, cam.Zoom))
	}

	// Returns before faces, so each face laps over the side it shows.
	for _, r := range [3]city.Rect{sign.Base, sign.Right, sign.Cornice} {
		g.returnOf(screen, r, depth, colorMonumentReturn)
	}

	g.fill(screen, sign.Base, colorMonumentStone)
	g.fill(screen, sign.Left, colorMonumentStone)
	g.fill(screen, sign.Right, colorMonumentStone)
	for _, c := range sign.Courses {
		g.fill(screen, c, colorMonumentCourse)
	}
	g.fill(screen, sign.Panel, colorMonumentPanel)
	g.fill(screen, sign.Reveal, colorMonumentReveal)
	g.fill(screen, sign.Soffit, colorMonumentSoffit)
	g.fill(screen, sign.Cornice, colorMonumentCornice)
	g.sign(screen, sign.Copy, colorMonumentCopy)

	// The planting last, in front, at the sign's foot.
	g.kit(screen, cam, kitPlanter, 0, bedOf(cam, at, width), nil)
}
