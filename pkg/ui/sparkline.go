package ui

import "botropolis.city/botropolis/pkg/city"

// Sparkline is a series laid along a box: the polyline to stroke, its
// last point to mark and the peak it was scaled to.
type Sparkline struct {
	Box    city.Rect
	Points []city.Point
	Last   city.Point
	Peak   float64
}

// LayoutSparkline scales values into a box, oldest at the left; a flat
// series lies along the floor.
func LayoutSparkline(th Theme, values []float64, box city.Rect) Sparkline {
	line := Sparkline{Box: box}
	if len(values) == 0 {
		return line
	}
	for _, v := range values {
		if v > line.Peak {
			line.Peak = v
		}
	}
	for i, v := range values {
		x := box.Max.X
		if len(values) > 1 {
			x = box.Min.X + box.Width()*float64(i)/float64(len(values)-1)
		}
		y := box.Max.Y
		if line.Peak > 0 {
			y = box.Max.Y - box.Height()*v/line.Peak
		}
		line.Points = append(line.Points, city.Point{X: x, Y: y})
	}
	line.Last = line.Points[len(line.Points)-1]
	return line
}
