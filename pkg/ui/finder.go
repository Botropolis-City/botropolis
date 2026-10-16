package ui

import (
	"image/color"

	"botropolis.city/botropolis/pkg/city"
)

// FinderInput is what the finder panel shows: the query as typed, with
// any composition already in it, the caret as a rune index into it, and
// a line under the field saying what the query found.
type FinderInput struct {
	Query  string
	Caret  int
	Status string
}

// FinderPanel is the finder laid out over the city: a plate holding the
// field on its first line and the status beneath it.
type FinderPanel struct {
	Rect   city.Rect
	Fill   color.NRGBA
	Radius float64
	Pad    float64
	Field  city.Rect
	Query  Text
	Caret  city.Rect
	Status Text
}

// finderGrids is the panel's width when the window has room for it.
const finderGrids = 80

// LayoutFinder centres the finder across a width×height window and
// hangs it in the top third, the way a command palette sits. A query
// wider than the field scrolls so the caret stays in view.
func LayoutFinder(th Theme, width, height float64, in FinderInput, measure Measure) FinderPanel {
	grid := th.Grid()
	pad := 2 * grid
	w := min(finderGrids*grid, width-2*grid)
	_, titleH := measure("", Title)
	_, smallH := measure("", Small)
	h := pad + titleH + grid + smallH + pad
	p := FinderPanel{
		Rect:   city.RectAt((width-w)/2, height/6, w, h),
		Fill:   th.Palette.Panel,
		Radius: th.Radius(),
		Pad:    pad,
	}
	p.Field = city.RectAt(p.Rect.Min.X+pad, p.Rect.Min.Y+pad, w-2*pad, titleH)
	caretW := 2 * th.Hairline()
	runes := []rune(in.Query)
	caret := max(0, min(in.Caret, len(runes)))
	start := 0
	for start < caret {
		if before, _ := measure(string(runes[start:caret]), Title); before <= p.Field.Width()-caretW {
			break
		}
		start++
	}
	before, _ := measure(string(runes[start:caret]), Title)
	shown := clipTo(string(runes[start:]), p.Field.Width(), Title, measure)
	p.Query = Text{Text: shown, At: p.Field.Min, Size: Title}
	p.Caret = city.RectAt(p.Field.Min.X+before, p.Field.Min.Y, caretW, titleH)
	p.Status = Text{Text: in.Status, At: city.Point{X: p.Field.Min.X, Y: p.Field.Max.Y + grid}, Size: Small}
	return p
}
