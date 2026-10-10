package ui

import "botropolis.city/botropolis/pkg/city"

// Help is the overlay listing every key: a titled panel of key chips
// with their actions in one column.
type Help struct {
	Rect  city.Rect
	Title Text
	Rows  []PlacedKey
}

// Hit finds the key under a screen point. A row's target is its whole
// band across the panel, chip and action together, so the thing you
// read is the thing you can press. Bug 43.
func (h Help) Hit(at city.Point) (Key, bool) {
	if !h.Rect.Contains(at) {
		return Key{}, false
	}
	for _, r := range h.Rows {
		if at.Y >= r.Chip.Min.Y && at.Y < r.Chip.Max.Y {
			return r.Key, true
		}
	}
	return Key{}, false
}

// LayoutHelp centres the key list in a width×height window.
func LayoutHelp(th Theme, width, height float64, title string, keys []Key, measure Measure) Help {
	grid := th.Grid()
	pad := 3 * grid
	_, lineH := measure("", Small)
	_, titleH := measure(title, Title)
	chipH := lineH + grid
	var chipW, labelW float64
	for _, k := range keys {
		if w, _ := measure(k.Key, Small); w+grid > chipW {
			chipW = w + grid
		}
		if w, _ := measure(k.Action, Small); w > labelW {
			labelW = w
		}
	}
	rowStep := chipH + grid/2
	if titleW, _ := measure(title, Title); titleW > chipW+grid+labelW {
		labelW = titleW - chipW - grid
	}
	w := pad + chipW + grid + labelW + pad
	h := pad + titleH + grid + rowStep*float64(len(keys)) - grid/2 + pad
	help := Help{Rect: city.RectAt((width-w)/2, (height-h)/2, w, h)}
	help.Title = Text{Text: title, At: help.Rect.Min.Add(city.Point{X: pad, Y: pad}), Size: Title}
	y := help.Rect.Min.Y + pad + titleH + grid
	for _, k := range keys {
		kw, _ := measure(k.Key, Small)
		chip := city.RectAt(help.Rect.Min.X+pad, y, kw+grid, chipH)
		help.Rows = append(help.Rows, PlacedKey{
			Key:     k,
			Chip:    chip,
			KeyAt:   city.Point{X: chip.Min.X + grid/2, Y: y + grid/2},
			LabelAt: city.Point{X: help.Rect.Min.X + pad + chipW + grid, Y: y + grid/2},
		})
		y += rowStep
	}
	return help
}
