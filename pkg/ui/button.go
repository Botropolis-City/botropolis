package ui

import "botropolis.city/botropolis/pkg/city"

// Button is a label on a chip that can be clicked.
type Button struct {
	Rect  city.Rect
	Label Text
}

// LayoutButtons lays labels out as a row of buttons starting at a point.
func LayoutButtons(th Theme, labels []string, at city.Point, measure Measure) []Button {
	grid := th.Grid()
	_, lineH := measure("", Body)
	x := at.X
	var row []Button
	for _, label := range labels {
		w, _ := measure(label, Body)
		row = append(row, Button{
			Rect:  city.RectAt(x, at.Y, w+2*grid, lineH+grid),
			Label: Text{Text: label, At: city.Point{X: x + grid, Y: at.Y + grid/2}, Size: Body},
		})
		x += w + 3*grid
	}
	return row
}

// LayoutButtonRows lays labels out as buttons starting at a point,
// wrapping onto a new row when one would run past maxWidth.
func LayoutButtonRows(th Theme, labels []string, at city.Point, maxWidth float64, measure Measure) []Button {
	grid := th.Grid()
	_, lineH := measure("", Body)
	x, y := at.X, at.Y
	var row []Button
	for _, label := range labels {
		w, _ := measure(label, Body)
		if x > at.X && x+w+2*grid > at.X+maxWidth {
			x, y = at.X, y+lineH+2*grid
		}
		row = append(row, Button{
			Rect:  city.RectAt(x, y, w+2*grid, lineH+grid),
			Label: Text{Text: label, At: city.Point{X: x + grid, Y: y + grid/2}, Size: Body},
		})
		x += w + 3*grid
	}
	return row
}

// HitButton is the button under a point, if any.
func HitButton(row []Button, at city.Point) (Button, bool) {
	for _, b := range row {
		if b.Rect.Contains(at) {
			return b, true
		}
	}
	return Button{}, false
}
