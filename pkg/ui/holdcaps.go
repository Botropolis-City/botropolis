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
// keys, one cap per press in a row whose right edge is the control's and
// which overlaps its top edge by half a grid, clear of its label, kept
// inside the window. A row that would cover one laid before it moves
// right, past it, so a row of buttons narrower than their caps still
// shows each cap whole. A control with no keys gets nothing.
func LayoutCaps(th Theme, controls []Control, window city.Rect, measure Measure) []HeldCaps {
	capH := LayoutKeyCap(th, "", city.Point{}, measure).Rect.Height()
	var held []HeldCaps
	var rows []city.Rect
	for _, c := range controls {
		if len(c.Keys) == 0 {
			continue
		}
		w := KeyCapsWidth(th, c.Keys, measure)
		at := city.Point{X: c.Rect.Max.X - w, Y: c.Rect.Min.Y + th.Grid()/2 - capH}
		for range len(rows) {
			r := city.RectAt(at.X, at.Y, w, capH)
			moved := false
			for _, other := range rows {
				if r.Overlaps(other) {
					at.X, moved = other.Max.X+th.Grid()/2, true
					break
				}
			}
			if !moved {
				break
			}
		}
		at.X = math.Max(window.Min.X, math.Min(at.X, window.Max.X-w))
		at.Y = math.Max(window.Min.Y, math.Min(at.Y, window.Max.Y-capH))
		rows = append(rows, city.RectAt(at.X, at.Y, w, capH))
		held = append(held, HeldCaps{Control: c, Caps: LayoutKeyCaps(th, c.Keys, at, measure)})
	}
	return held
}
