package ui

import "botropolis.city/botropolis/pkg/city"

// dialogLines is the most lines one line of a dialog's body wraps onto.
const dialogLines = 3

// dialogGrids is the widest a dialog's body grows to before it wraps.
const dialogGrids = 60

// DialogChoice is one button on a dialog and the keys that press it, as
// the text under the button spells them.
type DialogChoice struct {
	Label, Keys string
}

// Dialog is a prompt laid out: a plate in the middle of the window, its
// title, what the question means under it, a row of buttons and under
// each button the keys that press it. Anything with a choice or a loss
// asks this way (items 292, 352).
type Dialog struct {
	Plate   city.Rect
	Title   Text
	Lines   []Text
	Buttons []Button
	Keys    []Text
}

// LayoutDialog centres a dialog in a width×height window.
func LayoutDialog(th Theme, title string, lines []string, choices []DialogChoice, width, height float64, measure Measure) Dialog {
	grid := th.Grid()
	pad := 3 * grid
	_, lineH := measure("", Body)
	_, smallH := measure("", Small)
	widths := make([]float64, len(choices))
	row := 0.0
	for i, c := range choices {
		label, _ := measure(c.Label, Body)
		keys, _ := measure(c.Keys, Small)
		widths[i] = max(label, keys) + 4*grid
		row += widths[i]
	}
	if len(choices) > 1 {
		row += float64(len(choices)-1) * 2 * grid
	}
	titleW, _ := measure(title, Body)
	widest := max(row, titleW)
	for _, line := range lines {
		lw, _ := measure(line, Small)
		widest = max(widest, min(lw, dialogGrids*grid))
	}
	w := min(width-8*grid, widest+2*pad)
	inner := w - 2*pad
	var wrapped []string
	for _, line := range lines {
		wrapped = append(wrapped, wrapAt(line, inner, dialogLines, Small, measure)...)
	}
	body := smallH * float64(len(wrapped))
	if len(wrapped) > 0 {
		body += grid
	}
	buttonH := lineH + 2*grid
	h := pad + lineH + 2*grid + body + buttonH + grid + lineH + 2*grid
	plate := city.RectAt((width-w)/2, (height-h)/2, w, h)
	d := Dialog{Plate: plate}
	x0 := plate.Min.X + pad
	y := plate.Min.Y + pad
	d.Title = Text{Text: title, At: city.Point{X: plate.Center().X - titleW/2, Y: y}, Size: Body}
	y += lineH + 2*grid
	for _, line := range wrapped {
		d.Lines = append(d.Lines, Text{Text: line, At: city.Point{X: x0, Y: y}, Size: Small})
		y += smallH
	}
	if len(wrapped) > 0 {
		y += grid
	}
	x := plate.Min.X + (w-row)/2
	for i, c := range choices {
		label, _ := measure(c.Label, Body)
		keys, _ := measure(c.Keys, Small)
		d.Buttons = append(d.Buttons, Button{
			Rect:  city.RectAt(x, y, widths[i], buttonH),
			Label: Text{Text: c.Label, At: city.Point{X: x + (widths[i]-label)/2, Y: y + grid}, Size: Body},
		})
		d.Keys = append(d.Keys, Text{Text: c.Keys, At: city.Point{X: x + (widths[i]-keys)/2, Y: y + buttonH + grid}, Size: Small})
		x += widths[i] + 2*grid
	}
	return d
}
