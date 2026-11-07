package ui

import "botropolis.city/botropolis/pkg/city"

// Reading is the band along the window's bottom while a first launch
// reads the history: the line, and its bar filled as far as is read
// (items 159, 353).
type Reading struct {
	Rect      city.Rect
	Line      Text
	Bar, Fill city.Rect
}

// LayoutReading lays the reading band along the bottom of a width×height
// window; an empty line lays out nothing.
func LayoutReading(th Theme, width, height float64, line string, fraction float64, measure Measure) Reading {
	if line == "" {
		return Reading{}
	}
	grid := th.Grid()
	_, lineH := measure("", Small)
	barH := grid / 2
	top := height - (lineH + barH + 2*grid)
	r := Reading{Rect: city.RectAt(0, top, width, height-top)}
	r.Line = Text{Text: line, At: city.Point{X: 2 * grid, Y: top + grid/2}, Size: Small}
	r.Bar = city.RectAt(2*grid, top+grid/2+lineH+grid/2, width-4*grid, barH)
	r.Fill = city.RectAt(r.Bar.Min.X, r.Bar.Min.Y, r.Bar.Width()*min(max(fraction, 0), 1), barH)
	return r
}
