package ui

import "botropolis.city/botropolis/pkg/city"

// Text is one run of text placed on screen: what to draw, where its
// top-left corner goes, and at which of the four sizes.
type Text struct {
	Text string
	At   city.Point
	Size Size
}

func (t Text) moved(by city.Point) Text {
	t.At = t.At.Add(by)
	return t
}

// Plate is a line of text on a small panel: the tooltip, and the in-world
// label that never sits straight on the map.
type Plate struct {
	Rect   city.Rect
	TextAt city.Point
	Size   Size
}

// LayoutPlate wraps s in a plate whose top-left corner is at.
func LayoutPlate(th Theme, s string, at city.Point, size Size, measure Measure) Plate {
	w, h := measure(s, size)
	padX, padY := th.Grid()/2, th.Grid()/4
	return Plate{
		Rect:   city.RectAt(at.X, at.Y, w+2*padX, h+2*padY),
		TextAt: city.Point{X: at.X + padX, Y: at.Y + padY},
		Size:   size,
	}
}

// clipTo shortens s until it measures within width.
func clipTo(s string, width float64, size Size, measure Measure) string {
	for w, _ := measure(s, size); w > width && len(s) > 1; w, _ = measure(s, size) {
		runes := []rune(s)
		keep := int(float64(len(runes)) * width / w)
		if keep >= len(runes) {
			keep = len(runes) - 1
		}
		if keep < 1 {
			keep = 1
		}
		s = string(runes[:keep])
	}
	return s
}
