package shapes

import (
	"math"

	"botropolis.city/botropolis/pkg/city"
)

// tolerance is how far a flattened curve may stray from the curve, in
// pixels.
const tolerance = 0.25

// AppendQuad appends a quadratic Bézier curve from a through ctrl to b as
// points no further than a quarter pixel from it.
func AppendQuad(dst []city.Point, a, ctrl, b city.Point) []city.Point {
	// A quadratic's deviation from its chord is a quarter of the
	// control point's distance from the chord's midpoint, and halves
	// twice with every halving of the step, so n steps stray
	// dev/n² at most.
	dev := math.Hypot(ctrl.X-(a.X+b.X)/2, ctrl.Y-(a.Y+b.Y)/2) / 2
	n := max(1, int(math.Ceil(math.Sqrt(dev/tolerance))))
	dst = append(dst, a)
	for i := 1; i < n; i++ {
		s := float64(i) / float64(n)
		u := 1 - s
		dst = append(dst, city.Point{X: u*u*a.X + 2*u*s*ctrl.X + s*s*b.X, Y: u*u*a.Y + 2*u*s*ctrl.Y + s*s*b.Y})
	}
	return append(dst, b)
}

// AppendEllipse appends an ellipse round c as a closed ring of points no
// further than a quarter pixel from it.
func AppendEllipse(dst []city.Point, c city.Point, rx, ry float64) []city.Point {
	return AppendArc(dst, c, rx, ry, 0, 2*math.Pi, false)
}

// AppendArc appends the ellipse's arc from angle from to angle to; with
// last it ends on to's own point, otherwise it stops a step short, which
// is what a closed ring wants.
func AppendArc(dst []city.Point, c city.Point, rx, ry, from, to float64, last bool) []city.Point {
	n := arcSteps(math.Max(math.Abs(rx), math.Abs(ry)), math.Abs(to-from))
	end := n
	if last {
		end = n + 1
	}
	for i := range end {
		t := from + (to-from)*float64(i)/float64(n)
		dst = append(dst, city.Point{X: c.X + rx*math.Cos(t), Y: c.Y + ry*math.Sin(t)})
	}
	return dst
}

// AppendRoundRect appends a rectangle with corners rounded to radius,
// no more than half its shorter side, as a closed ring of points.
func AppendRoundRect(dst []city.Point, r city.Rect, radius float64) []city.Point {
	x0, y0, x1, y1 := r.Min.X, r.Min.Y, r.Max.X, r.Max.Y
	rad := math.Max(0, math.Min(radius, math.Min(x1-x0, y1-y0)/2))
	if rad == 0 {
		return append(dst, city.Point{X: x0, Y: y0}, city.Point{X: x1, Y: y0}, city.Point{X: x1, Y: y1}, city.Point{X: x0, Y: y1})
	}
	dst = AppendArc(dst, city.Point{X: x1 - rad, Y: y0 + rad}, rad, rad, -math.Pi/2, 0, true)
	dst = AppendArc(dst, city.Point{X: x1 - rad, Y: y1 - rad}, rad, rad, 0, math.Pi/2, true)
	dst = AppendArc(dst, city.Point{X: x0 + rad, Y: y1 - rad}, rad, rad, math.Pi/2, math.Pi, true)
	return AppendArc(dst, city.Point{X: x0 + rad, Y: y0 + rad}, rad, rad, math.Pi, 3*math.Pi/2, true)
}

func arcSteps(r, sweep float64) int {
	if r <= tolerance {
		return max(3, int(math.Ceil(sweep/(2*math.Pi)*6)))
	}
	step := 2 * math.Acos(1-tolerance/r)
	return max(3, int(math.Ceil(sweep/step)))
}
