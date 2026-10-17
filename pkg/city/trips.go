package city

import "math"

// Route is the way out of a building: the door, the service lane beside
// it, and the kerb where the block meets the avenue. Every point is
// inside the district's own block, so nothing that follows it drives on
// an avenue or through the building in front. The inspector parks at its
// end, and a delivery truck sets off from there.
func (d *District) Route(b *Building) []Point {
	door := d.Door(b)
	if d == nil {
		return []Point{door}
	}
	lane := clampTo(b.Rect.Max.X+BuildingGap/2, d.Rect.Min.X+DoorInset, d.Rect.Max.X-DoorInset)
	kerb := math.Max(door.Y, d.Rect.Max.Y-DoorInset)
	return []Point{door, {X: lane, Y: door.Y}, {X: lane, Y: kerb}}
}

func hypot(a, b Point) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }

// alongPath is the point a fraction of the way down a polyline.
func alongPath(path []Point, p float64) Point {
	if len(path) == 0 {
		return Point{}
	}
	if len(path) == 1 {
		return path[0]
	}
	total := 0.0
	for i := 1; i < len(path); i++ {
		total += hypot(path[i], path[i-1])
	}
	if total == 0 {
		return path[0]
	}
	want := math.Min(1, math.Max(0, p)) * total
	for i := 1; i < len(path); i++ {
		leg := hypot(path[i], path[i-1])
		if want <= leg || i == len(path)-1 {
			at := 0.0
			if leg > 0 {
				at = math.Min(1, want/leg)
			}
			return Point{
				X: path[i-1].X + (path[i].X-path[i-1].X)*at,
				Y: path[i-1].Y + (path[i].Y-path[i-1].Y)*at,
			}
		}
		want -= leg
	}
	return path[len(path)-1]
}
