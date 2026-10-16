package ui

import "botropolis.city/botropolis/pkg/city"

// LensPanel is a lens introducing itself (item 88): its name, large;
// what it shows; its legend with units at both ends; the aggregate; and
// what to do. Once seen it shrinks to a chip of its name.
type LensPanel struct {
	Rect  city.Rect
	Name  Text
	About []Text
	// Bar is the ramp, Low and High its two ends; Swatches and Labels
	// are a categorical legend instead.
	Bar       city.Rect
	Low, High Text
	Swatches  []city.Rect
	Labels    []Text
	Note      Text
	Aggregate Text
	Do        []Text
	Chip      bool
}

// LayoutLensPanel places the panel in the top-left corner of bounds.
func LayoutLensPanel(th Theme, p city.LensPanel, bounds city.Rect, measure Measure) LensPanel {
	grid := th.Grid()
	margin := 2 * grid
	at := bounds.Min.Add(city.Point{X: margin, Y: margin})
	if p.Chip {
		w, h := measure(p.Name, Small)
		return LensPanel{
			Rect: city.RectAt(at.X, at.Y, w+2*grid, h+grid),
			Name: Text{Text: p.Name, At: at.Add(city.Point{X: grid, Y: grid / 2}), Size: Small},
			Chip: true,
		}
	}
	pad := 2 * grid
	width := min(ParaWidth(th)*0.75, bounds.Width()-2*margin-2*pad)
	x, y := at.X+pad, at.Y+pad
	l := LensPanel{}
	_, nameH := measure(p.Name, Display)
	l.Name = Text{Text: p.Name, At: city.Point{X: x, Y: y}, Size: Display}
	y += nameH + grid
	_, bodyH := measure("", Body)
	_, smallH := measure("", Small)
	for _, line := range wrapPara(p.About, width, 3, measure) {
		l.About = append(l.About, Text{Text: line, At: city.Point{X: x, Y: y}, Size: Body})
		y += bodyH
	}
	switch lg := p.Legend; {
	case lg.Shown && lg.Ramp:
		y += grid
		lowW, _ := measure(lg.Low, Small)
		highW, _ := measure(lg.High, Small)
		l.Low = Text{Text: lg.Low, At: city.Point{X: x, Y: y}, Size: Small}
		barX := x + lowW + grid
		barW := max(4*grid, width-lowW-highW-2*grid)
		l.Bar = city.RectAt(barX, y+smallH/4, barW, smallH/2)
		l.High = Text{Text: lg.High, At: city.Point{X: barX + barW + grid, Y: y}, Size: Small}
		y += smallH
	case lg.Shown && len(lg.Categories) > 0:
		y += grid
		cx := x
		for _, name := range lg.Categories {
			w, _ := measure(name, Small)
			l.Swatches = append(l.Swatches, city.RectAt(cx, y+smallH/4, grid, smallH/2))
			l.Labels = append(l.Labels, Text{Text: name, At: city.Point{X: cx + grid + grid/2, Y: y}, Size: Small})
			cx += grid + grid/2 + w + 2*grid
		}
		y += smallH
	case lg.Note != "":
		y += grid
		l.Note = Text{Text: lg.Note, At: city.Point{X: x, Y: y}, Size: Small}
		y += smallH
	}
	if p.Aggregate != "" {
		y += grid
		l.Aggregate = Text{Text: p.Aggregate, At: city.Point{X: x, Y: y}, Size: Body}
		y += bodyH
	}
	if p.Do != "" {
		y += grid / 2
		for _, line := range wrapPara(p.Do, width, 2, measure) {
			l.Do = append(l.Do, Text{Text: line, At: city.Point{X: x, Y: y}, Size: Small})
			y += smallH
		}
	}
	l.Rect = city.RectAt(at.X, at.Y, width+2*pad, y+pad-at.Y)
	return l
}
