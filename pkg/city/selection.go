package city

import (
	"math"
	"sort"
)

// Districts selected to move together: toggled one at a time, or every
// district a box on the screen touches. Item 283.

// ToggleDistrict adds the district under a screen point to the selection,
// or takes it out, and reports whether there was one.
func (s *Scene) ToggleDistrict(screen Point) bool {
	s.PointerMove(screen)
	d := s.hover.District
	if d == nil {
		return false
	}
	if _, ok := s.city.plan.Slots[d.Root]; !ok {
		return false
	}
	if s.chosen[d.Root] {
		delete(s.chosen, d.Root)
		return true
	}
	if s.chosen == nil {
		s.chosen = map[string]bool{}
	}
	s.chosen[d.Root] = true
	return true
}

// SelectBox adds every district a screen box touches to the selection.
func (s *Scene) SelectBox(a, b Point) {
	for _, root := range s.DistrictsInBox(a, b) {
		if s.chosen == nil {
			s.chosen = map[string]bool{}
		}
		s.chosen[root] = true
	}
}

// DistrictsInBox is every district whose ground a screen box touches.
func (s *Scene) DistrictsInBox(a, b Point) []string {
	lo := Point{X: math.Min(a.X, b.X), Y: math.Min(a.Y, b.Y)}
	hi := Point{X: math.Max(a.X, b.X), Y: math.Max(a.Y, b.Y)}
	var roots []string
	for _, d := range s.city.Districts {
		if _, ok := s.city.plan.Slots[d.Root]; !ok {
			continue
		}
		r := d.Rect
		quad := []Point{
			s.camera.WorldToScreen(r.Min),
			s.camera.WorldToScreen(Point{X: r.Max.X, Y: r.Min.Y}),
			s.camera.WorldToScreen(r.Max),
			s.camera.WorldToScreen(Point{X: r.Min.X, Y: r.Max.Y}),
		}
		if quadTouchesBox(quad, lo, hi) {
			roots = append(roots, d.Root)
		}
	}
	sort.Strings(roots)
	return roots
}

// quadTouchesBox separates a convex quad from a box along the box's two
// axes and the quad's edge normals; touching is having no separating axis.
func quadTouchesBox(quad []Point, lo, hi Point) bool {
	box := []Point{lo, {X: hi.X, Y: lo.Y}, hi, {X: lo.X, Y: hi.Y}}
	axes := []Point{{X: 1}, {Y: 1}}
	for i, p := range quad {
		q := quad[(i+1)%len(quad)]
		axes = append(axes, Point{X: p.Y - q.Y, Y: q.X - p.X})
	}
	for _, axis := range axes {
		qlo, qhi := project(quad, axis)
		blo, bhi := project(box, axis)
		if qhi < blo || bhi < qlo {
			return false
		}
	}
	return true
}

func project(points []Point, axis Point) (float64, float64) {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, p := range points {
		v := p.X*axis.X + p.Y*axis.Y
		lo, hi = math.Min(lo, v), math.Max(hi, v)
	}
	return lo, hi
}

// ClearSelection selects no district.
func (s *Scene) ClearSelection() { s.chosen = nil }

// SelectedDistricts is the selection's districts that are still in the
// city, in order.
func (s *Scene) SelectedDistricts() []string {
	var roots []string
	for _, d := range s.city.Districts {
		if s.chosen[d.Root] {
			roots = append(roots, d.Root)
		}
	}
	sort.Strings(roots)
	return roots
}

// DistrictSelected reports whether a district is in the selection.
func (s *Scene) DistrictSelected(root string) bool { return s.chosen[root] }
