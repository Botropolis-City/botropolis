package ui

import (
	"math"

	"botropolis.city/botropolis/pkg/city"
)

// Control is something on screen that a key also does, where it is
// drawn, and the presses that do it (item 351).
type Control struct {
	// Name says which control it is, for the frame's probe.
	Name string
	Rect city.Rect
	Keys []string
}

// HeldCaps is a control's keys as laid while the hold key is down.
type HeldCaps struct {
	Control
	Caps []KeyCap
}

// LayoutCaps lays the hold's caps (item 351): for each control with
// keys, one cap per press in a row. A cap never covers text, so the row
// goes in the first of four places that covers none of text: at the
// control's top right, overlapping its top edge by half a grid, clear of
// its label; at the right end inside the control's own row; at its
// left, on its row; under it, overlapping its bottom edge by half a
// grid. Where all four cover text it takes the one covering least. A row
// that would cover one laid before it moves right, past it, so a row of
// buttons narrower than their caps still shows each cap whole. Every row
// is kept inside the window. A control with no keys gets nothing.
func LayoutCaps(th Theme, controls []Control, text []city.Rect, window city.Rect, measure Measure) []HeldCaps {
	grid := th.Grid()
	capH := LayoutKeyCap(th, "", city.Point{}, measure).Rect.Height()
	var held []HeldCaps
	var rows []city.Rect
	covers := func(r city.Rect, avoid []city.Rect) bool {
		for _, t := range avoid {
			if r.Overlaps(t) {
				return true
			}
		}
		return false
	}
	inside := func(at city.Point, w float64) city.Point {
		at.X = math.Max(window.Min.X, math.Min(at.X, window.Max.X-w))
		at.Y = math.Max(window.Min.Y, math.Min(at.Y, window.Max.Y-capH))
		return at
	}
	for _, c := range controls {
		if len(c.Keys) == 0 {
			continue
		}
		w := KeyCapsWidth(th, c.Keys, measure)
		middle := c.Rect.Center().Y - capH/2
		at := city.Point{X: c.Rect.Max.X - w, Y: c.Rect.Min.Y + grid/2 - capH}
		for range len(rows) {
			r := city.RectAt(at.X, at.Y, w, capH)
			moved := false
			for _, other := range rows {
				if r.Overlaps(other) {
					at.X, moved = other.Max.X+grid/2, true
					break
				}
			}
			if !moved {
				break
			}
		}
		at = inside(at, w)
		least := math.Inf(1)
		first := at
		for _, place := range []city.Point{
			at,
			{X: c.Rect.Max.X - grid/2 - w, Y: middle},
			{X: c.Rect.Min.X - grid/2 - w, Y: middle},
			{X: first.X, Y: c.Rect.Max.Y - grid/2},
		} {
			place = inside(place, w)
			r := city.RectAt(place.X, place.Y, w, capH)
			if covers(r, rows) {
				continue
			}
			if covered := coveredArea(r, text); covered < least {
				at, least = place, covered
			}
			if least == 0 {
				break
			}
		}
		rows = append(rows, city.RectAt(at.X, at.Y, w, capH))
		held = append(held, HeldCaps{Control: c, Caps: LayoutKeyCaps(th, c.Keys, at, measure)})
	}
	return held
}

// coveredArea is how much of the text a rect covers.
func coveredArea(r city.Rect, text []city.Rect) float64 {
	area := 0.0
	for _, t := range text {
		if r.Overlaps(t) {
			area += city.Rect{Min: city.Point{X: max(r.Min.X, t.Min.X), Y: max(r.Min.Y, t.Min.Y)}, Max: city.Point{X: min(r.Max.X, t.Max.X), Y: min(r.Max.Y, t.Max.Y)}}.Area()
		}
	}
	return area
}

// TextRects is every run of text on the card, where it is drawn, for
// the hold's caps to keep off (item 351).
func (c Card) TextRects(measure Measure) []city.Rect {
	runs := []Text{c.Title, c.Kind, c.Move, c.Group, c.HintDot}
	for _, lines := range [][]Text{c.Sentence, c.Numbers, c.Details, c.Hint, c.Lines, c.Para} {
		runs = append(runs, lines...)
	}
	for _, b := range c.Buttons {
		runs = append(runs, b.Label)
	}
	var rects []city.Rect
	for _, t := range runs {
		if t.Text == "" {
			continue
		}
		w, h := measure(t.Text, t.Size)
		rects = append(rects, city.RectAt(t.At.X, t.At.Y, w, h))
	}
	return rects
}
