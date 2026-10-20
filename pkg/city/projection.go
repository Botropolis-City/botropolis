package city

import "math"

// Projection maps world units onto the map plane. TopDown is the identity;
// Isometric is the classic 2:1 view, an affine map, so districts stay
// rectangles in the world and only the renderer sees diamonds.
type Projection int

const (
	TopDown Projection = iota
	Isometric
)

// IsoScale makes one building footprint (BuildingSize world units square)
// project to a diamond IsoTileWidth pixels wide at zoom 1, so Kenney's
// isometric tiles draw at native size there.
const (
	IsoTileWidth = 132.0
	IsoScale     = IsoTileWidth / (2 * BuildingSize)
)

func ParseProjection(s string) (Projection, bool) {
	switch s {
	case "top", "top-down", "topdown":
		return TopDown, true
	case "iso", "isometric":
		return Isometric, true
	}
	return TopDown, false
}

func (p Projection) String() string {
	if p == Isometric {
		return "iso"
	}
	return "top"
}

// Apply takes a world point onto the map plane.
func (p Projection) Apply(w Point) Point {
	if p != Isometric {
		return w
	}
	return Point{X: (w.X - w.Y) * IsoScale, Y: (w.X + w.Y) * IsoScale / 2}
}

// Invert takes a map-plane point back to the world.
func (p Projection) Invert(m Point) Point {
	if p != Isometric {
		return m
	}
	x := m.X / IsoScale
	y := 2 * m.Y / IsoScale
	return Point{X: (y + x) / 2, Y: (y - x) / 2}
}

// Bounds is the map-plane box around a projected world rectangle.
func (p Projection) Bounds(r Rect) Rect {
	corners := []Point{
		p.Apply(r.Min),
		p.Apply(Point{X: r.Max.X, Y: r.Min.Y}),
		p.Apply(r.Max),
		p.Apply(Point{X: r.Min.X, Y: r.Max.Y}),
	}
	out := Rect{Min: corners[0], Max: corners[0]}
	for _, c := range corners[1:] {
		out.Min.X = math.Min(out.Min.X, c.X)
		out.Min.Y = math.Min(out.Min.Y, c.Y)
		out.Max.X = math.Max(out.Max.X, c.X)
		out.Max.Y = math.Max(out.Max.Y, c.Y)
	}
	return out
}

// Corners is a world rectangle's outline on the map plane: top, right,
// bottom, left for the isometric diamond.
func (p Projection) Corners(r Rect) [4]Point {
	return [4]Point{
		p.Apply(r.Min),
		p.Apply(Point{X: r.Max.X, Y: r.Min.Y}),
		p.Apply(r.Max),
		p.Apply(Point{X: r.Min.X, Y: r.Max.Y}),
	}
}
