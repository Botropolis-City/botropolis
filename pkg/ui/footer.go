package ui

import "botropolis.city/botropolis/pkg/city"

// Key is one binding for the key row: the key as the user types it and
// what it does.
type Key struct {
	Key    string
	Action string
}

// PlacedKey is a key's cap and its label on the footer.
type PlacedKey struct {
	Key
	Cap     KeyCap
	LabelAt city.Point
}

// Footer is the bar along the bottom: the key row. KeyRow is the key
// row's part, which is all the map ever makes room for. A passing
// message is a notice under the strip, never a line here (item 352).
type Footer struct {
	Rect   city.Rect
	KeyRow city.Rect
	Size   Size
	Keys   []PlacedKey
	// Reading is the line while the history is read, over ReadingBar,
	// filled as far as ReadingFill; empty when nothing is. Item 159.
	Reading                 Text
	ReadingBar, ReadingFill city.Rect
}

// Hit finds the footer verb under a screen point, so a verb can be
// clicked as well as typed. Bug 43: a verb that can only be typed
// teaches nothing, and one that can be clicked teaches its own key.
//
// A verb's target is its chip and the label beside it, out to wherever
// the next chip starts — the label's width is not kept, and the gap
// between verbs belongs to whichever verb is on its left.
func (f Footer) Hit(at city.Point) (Key, bool) {
	if !f.KeyRow.Contains(at) {
		return Key{}, false
	}
	for i, k := range f.Keys {
		right := f.Rect.Max.X
		if i+1 < len(f.Keys) {
			right = f.Keys[i+1].Cap.Rect.Min.X
		}
		if at.X >= k.Cap.Rect.Min.X && at.X < right {
			return k.Key, true
		}
	}
	return Key{}, false
}

// LayoutFooter lays the footer along the bottom of a width×height window.
func LayoutFooter(th Theme, width, height float64, keys []Key, measure Measure) Footer {
	grid := th.Grid()
	f := Footer{Size: Small}
	_, lineH := measure("", Small)
	f.KeyRow = city.RectAt(0, height-lineH-2*grid, width, lineH+2*grid)
	f.Rect = f.KeyRow
	x := 2 * grid
	for _, k := range keys {
		lw, _ := measure(k.Action, Small)
		cap := LayoutKeyCap(th, k.Key, city.Point{X: x, Y: f.KeyRow.Min.Y + grid/2}, measure)
		labelX := cap.Rect.Max.X + grid
		if labelX+lw > width-2*grid {
			break
		}
		f.Keys = append(f.Keys, PlacedKey{
			Key:     k,
			Cap:     cap,
			LabelAt: city.Point{X: labelX, Y: f.KeyRow.Min.Y + grid},
		})
		x = labelX + lw + 3*grid
	}
	return f
}
