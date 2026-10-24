package city

import "math"

const (
	MinZoom    = 0.25
	FitMinZoom = 0.02
	// MaxZoom is where the sprites run out, not where the maths does.
	// pkg/assets/kits holds two levels, z1 at tile 132 and z2 at tile
	// 264, so past 2 there is nothing to draw and z2 is stretched —
	// which is what "the tower looks low res" was, map-wide, read off
	// the one piece whose smooth curved surface could not hide it.
	//
	// The ladder used to go to 4. Cutting z3 and z4 was budgeted first
	// and does not fit: pages grow with the square of the zoom, so z3
	// wants 14 pages and z4 wants 23 against a budget of 8 each, on top
	// of z2's 6. A ladder that promises a zoom the art cannot serve is
	// the same lie as a label that does not match its behaviour, so the
	// ladder stops where the art does. pkg/assets'
	// TestZoomLadderStopsWhereTheAtlasDoes holds the two together.
	//
	// Since item 162's slice 14 the closest level is z1.5, drawn up to 2
	// at 1.33x, in place of z2: Aria could not tell them apart
	// (2026-10-24), and it is 26 MiB less texture. So "where the art
	// does" is a draw-up below assets.KitDrawUpMost, not a cut at 2.
	MaxZoom = 2.0
	fitPad  = 40.0
	// FitFooter keeps the bottom strip, where the footer draws, clear of the city.
	FitFooter = 40.0
)

// ZoomSteps is the wheel's ladder: 16 px tiles land on whole screen
// pixels at 1 and 2, and the steps below are where the map view takes
// over from sprites. It ends at MaxZoom because that is where the atlas
// ends; see the note there.
var ZoomSteps = []float64{MinZoom, 0.35, 0.5, 0.75, 1, 1.5, MaxZoom}

type Camera struct {
	Offset     Point
	Zoom       float64
	Projection Projection
	// Heading is the way the camera faces, in degrees, one of four; the
	// sprites are cut for each.
	Heading int
}

func NewCamera() *Camera {
	return &Camera{Zoom: 1}
}

func (c *Camera) Pan(delta Point) {
	c.Offset = c.Offset.Add(delta)
}

func (c *Camera) WorldToScreen(p Point) Point {
	return c.Project(p).Add(c.Offset).Scale(c.Zoom)
}

func (c *Camera) ScreenToWorld(p Point) Point {
	return c.Unproject(p.Scale(1 / c.Zoom).Sub(c.Offset))
}

// turn spins a world point a quarter turn per 90 degrees of heading
// about the origin, so the same map can be looked at from four sides.
func (c *Camera) turn(p Point) Point {
	switch ((c.Heading/90)%4 + 4) % 4 {
	case 1:
		return Point{X: -p.Y, Y: p.X}
	case 2:
		return Point{X: -p.X, Y: -p.Y}
	case 3:
		return Point{X: p.Y, Y: -p.X}
	}
	return p
}

// TurnRect is a world rectangle in the camera's own frame. A heading is
// a quarter turn, so the result is still axis aligned and turning the
// two opposite corners is enough.
func (c *Camera) TurnRect(r Rect) Rect {
	a, b := c.turn(r.Min), c.turn(r.Max)
	return Rect{
		Min: Point{X: math.Min(a.X, b.X), Y: math.Min(a.Y, b.Y)},
		Max: Point{X: math.Max(a.X, b.X), Y: math.Max(a.Y, b.Y)},
	}
}

// Behind reports whether a is hidden by b: in the camera's own frame, a
// ends before b begins on one of the two axes.
//
// Bug 42. DepthOf keys a footprint at one corner, and no single scalar
// can order a point against a box here — key the box at its back corner
// and everything standing behind it draws over it, key it at its front
// corner and it draws over everything in front. A 4x4 building spans
// eight units of depth and was being compared as though it were a point
// at its far corner, so a tree on the cell behind its back edge sorted
// after it and was painted across the facade.
//
// This is a separating axis, and it is only a partial order: two
// footprints that overlap on both axes are behind neither, which on a
// grid of non-overlapping footprints means they do not hide each other
// and any order will do.
func (c *Camera) Behind(a, b Rect) bool {
	return BehindTurned(c.TurnRect(a), c.TurnRect(b))
}

// BehindTurned is Behind for rectangles already in the camera's frame,
// for a sort that turns them once rather than once per comparison. It
// is the same expression, so the two cannot drift.
func BehindTurned(a, b Rect) bool {
	return a.Max.X <= b.Min.X || a.Max.Y <= b.Min.Y
}

// TurnPoint is a world point in the camera's own frame, where depth is
// x + y. Exported for the draw-order guard, which needs to say what
// "behind" means from a heading.
func (c *Camera) TurnPoint(p Point) Point { return c.turn(p) }

func (c *Camera) unturn(p Point) Point {
	switch ((c.Heading/90)%4 + 4) % 4 {
	case 1:
		return Point{X: p.Y, Y: -p.X}
	case 2:
		return Point{X: -p.X, Y: -p.Y}
	case 3:
		return Point{X: -p.Y, Y: p.X}
	}
	return p
}

// Project takes a world point onto the map plane as the camera faces it.
func (c *Camera) Project(p Point) Point {
	return c.Projection.Apply(c.turn(p))
}

// Unproject takes a map-plane point back to the world.
func (c *Camera) Unproject(m Point) Point {
	return c.unturn(c.Projection.Invert(m))
}

// Corners is a world rectangle's outline on the map plane as the camera
// faces it, in the rectangle's own corner order.
func (c *Camera) Corners(r Rect) [4]Point {
	return [4]Point{
		c.Project(r.Min),
		c.Project(Point{X: r.Max.X, Y: r.Min.Y}),
		c.Project(r.Max),
		c.Project(Point{X: r.Min.X, Y: r.Max.Y}),
	}
}

// Bounds is the map-plane box around a projected world rectangle.
func (c *Camera) Bounds(r Rect) Rect {
	corners := c.Corners(r)
	out := Rect{Min: corners[0], Max: corners[0]}
	for _, p := range corners[1:] {
		out.Min.X = math.Min(out.Min.X, p.X)
		out.Min.Y = math.Min(out.Min.Y, p.Y)
		out.Max.X = math.Max(out.Max.X, p.X)
		out.Max.Y = math.Max(out.Max.Y, p.Y)
	}
	return out
}

// Depth orders what is drawn back to front for this heading: the
// larger, the nearer the viewer.
func (c *Camera) Depth(p Point) float64 {
	t := c.turn(p)
	return t.X + t.Y
}

// DepthOf orders a footprint back to front: the depth of its back-most
// corner, the first part of it the viewer would lose sight of.
//
// One point is only right for something standing on one cell. A piece
// that spans cells — the plant and its cooling tower — keyed off a
// single corner ties with whatever sits on the cell beside it, and the
// order between them is then whatever the sort happened to do. Sorting
// by where a footprint begins breaks the tie the way the eye does.
func (c *Camera) DepthOf(r Rect) float64 {
	depth := math.Inf(1)
	for _, p := range [4]Point{r.Min, {X: r.Max.X, Y: r.Min.Y}, r.Max, {X: r.Min.X, Y: r.Max.Y}} {
		depth = math.Min(depth, c.Depth(p))
	}
	return depth
}

// Turn faces the camera a quarter turn on, keeping the world point under
// a screen point where it is.
func (c *Camera) Turn(quarters int, pivot Point) {
	world := c.ScreenToWorld(pivot)
	c.Heading = ((c.Heading+90*quarters)%360 + 360) % 360
	c.Offset = pivot.Scale(1 / c.Zoom).Sub(c.Project(world))
}

// ZoomAt steps the zoom up (factor > 1) or down the ladder, keeping the
// world point under the cursor where it is.
func (c *Camera) ZoomAt(cursor Point, factor float64) {
	if factor > 1 {
		c.SetZoomAt(cursor, stepAbove(c.Zoom))
	} else {
		c.SetZoomAt(cursor, stepBelow(c.Zoom))
	}
}

// SetZoomAt sets the zoom, keeping the world point under the cursor
// where it is.
func (c *Camera) SetZoomAt(cursor Point, zoom float64) {
	before := c.ScreenToWorld(cursor)
	c.Zoom = zoom
	after := c.ScreenToWorld(cursor)
	// Offset lives on the map plane, so the world shift is projected first.
	c.Offset = c.Offset.Add(c.Project(after).Sub(c.Project(before)))
}

const zoomEpsilon = 1e-9

func stepAbove(zoom float64) float64 {
	for _, step := range ZoomSteps {
		if step > zoom+zoomEpsilon {
			return step
		}
	}
	return MaxZoom
}

func stepBelow(zoom float64) float64 {
	for i := len(ZoomSteps) - 1; i >= 0; i-- {
		if ZoomSteps[i] < zoom-zoomEpsilon {
			return ZoomSteps[i]
		}
	}
	return math.Min(zoom, MinZoom)
}

// snapDown is the largest ladder step that still fits; below the ladder
// the zoom stays continuous so a huge city can still be shown whole.
func snapDown(zoom float64) float64 {
	snapped := zoom
	for _, step := range ZoomSteps {
		if step <= zoom+zoomEpsilon {
			snapped = step
		}
	}
	return snapped
}

func (c *Camera) Fit(bounds Rect, width, height float64) {
	c.FitWithInsets(bounds, width, height, Insets{Bottom: FitFooter})
}

// Insets are screen-space margins kept clear of the city when fitting: the
// footer at the bottom, tower labels on the left, landmark labels on top.
type Insets struct {
	Left, Top, Right, Bottom float64
}

func (c *Camera) FitWithInsets(bounds Rect, width, height float64, in Insets) {
	bounds = c.Bounds(bounds)
	if bounds.Width() <= 0 || bounds.Height() <= 0 || width <= 0 || height <= 0 {
		return
	}
	availW := width - in.Left - in.Right - 2*fitPad
	availH := height - in.Top - in.Bottom - 2*fitPad
	if availW <= 0 || availH <= 0 {
		return
	}
	// Fitting may go below MinZoom: the wheel floor stops the user losing the
	// city, but a short tiled window still has to show all of it.
	c.Zoom = math.Min(MaxZoom, math.Max(FitMinZoom, math.Min(availW/bounds.Width(), availH/bounds.Height())))
	if c.Projection == TopDown {
		// Pixel art wants whole pixels per tile; the isometric renders are
		// smooth-shaded and scale freely.
		c.Zoom = snapDown(c.Zoom)
	}
	centre := bounds.Center()
	screenCentre := Point{X: in.Left + (width-in.Left-in.Right)/2, Y: in.Top + (height-in.Top-in.Bottom)/2}
	c.Offset = screenCentre.Scale(1 / c.Zoom).Sub(centre)
}
