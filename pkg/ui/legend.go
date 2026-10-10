package ui

import (
	"image/color"

	"botropolis.city/botropolis/pkg/city"
)

// The legend a view is not shippable without.
//
// A view paints the city by a number or a kind, and a colour standing for
// a number means nothing without the number beside it. This is that, laid
// out: the view's name, then either a swatch and a label per category or
// a gradient bar with its two ends written out.
//
// It sits above the key row and takes no room at all when no view is up,
// so entering and leaving a view never moves the map.

// LegendEntry is one category: its colour, its swatch and its name.
type LegendEntry struct {
	Label   string
	Swatch  color.NRGBA
	Box     city.Rect
	LabelAt city.Point
}

// Legend is the whole thing placed for one window.
type Legend struct {
	Rect    city.Rect
	Size    Size
	Title   Text
	Entries []LegendEntry
	// Bar is the gradient a ramp view draws between Low and High; it has
	// no area on a categorical view.
	Bar       city.Rect
	Low, High Text
	// Aside is the second fact, placed after everything else.
	Aside Text
}

// barWidth is how long the gradient runs, in grid squares. Long enough
// to read as a scale rather than a swatch.
const barWidth = 12

// LayoutLegend places the legend as one row directly above bottom, which
// is the top of whatever the footer laid out. Stacking it against the
// footer rather than against the window is what keeps a passing status
// from landing on top of it.
func LayoutLegend(th Theme, width, bottom float64, spec city.Legend, measure Measure) Legend {
	if !spec.Shown {
		return Legend{}
	}
	grid := th.Grid()
	l := Legend{Size: Small}
	_, lineH := measure("", Small)
	top := bottom - lineH - grid
	l.Rect = city.RectAt(0, top, width, lineH+grid)
	baseline := top + grid/2

	x := 2 * grid
	titleW, _ := measure(spec.Title, Small)
	l.Title = Text{Text: spec.Title, At: city.Point{X: x, Y: baseline}, Size: Small}
	x += titleW + 2*grid

	aside := func(at float64) {
		if spec.Aside == "" {
			return
		}
		l.Aside = Text{Text: spec.Aside, At: city.Point{X: at + 3*grid, Y: baseline}, Size: Small}
	}
	if spec.Note != "" {
		w, _ := measure(spec.Note, Small)
		l.Low = Text{Text: spec.Note, At: city.Point{X: x, Y: baseline}, Size: Small}
		aside(x + w)
		return l
	}
	if spec.Ramp {
		lowW, _ := measure(spec.Low, Small)
		l.Low = Text{Text: spec.Low, At: city.Point{X: x, Y: baseline}, Size: Small}
		x += lowW + grid
		l.Bar = city.RectAt(x, baseline+lineH/4, barWidth*grid, lineH/2)
		x = l.Bar.Max.X + grid
		l.High = Text{Text: spec.High, At: city.Point{X: x, Y: baseline}, Size: Small}
		highW, _ := measure(spec.High, Small)
		aside(x + highW)
		return l
	}

	for i, name := range spec.Categories {
		label := clipTo(name, width/4, Small, measure)
		w, _ := measure(label, Small)
		box := city.RectAt(x, baseline+lineH/4, grid, grid)
		if box.Max.X+grid+w > width-2*grid {
			break
		}
		l.Entries = append(l.Entries, LegendEntry{
			Label:   label,
			Swatch:  swatchFor(spec, i),
			Box:     box,
			LabelAt: city.Point{X: box.Max.X + grid, Y: baseline},
		})
		x = box.Max.X + grid + w + 2*grid
	}
	return l
}

// swatchFor is the colour the nth category is painted. "Other" is the
// last entry and is the neutral, which Category already returns for any
// index past the palette — but naming it here is what makes the legend
// agree with the map rather than happening to.
func swatchFor(spec city.Legend, i int) color.NRGBA {
	if spec.Categories[i] == city.OtherCategory {
		return Uncategorised
	}
	return Category(i)
}
