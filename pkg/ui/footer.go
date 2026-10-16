package ui

import (
	"strings"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/format"
)

const maxFooterLines = 4

// Key is one binding for the key row: the key as the user types it and
// what it does.
type Key struct {
	Key    string
	Action string
}

// PlacedKey is a key chip and its label on the footer.
type PlacedKey struct {
	Key
	Chip    city.Rect
	KeyAt   city.Point
	LabelAt city.Point
}

// Footer is the bar along the bottom: the key row, with a status on its
// own lines above it when there is one. KeyRow is the key row's part,
// which is all the map ever makes room for, so a passing status never
// moves the map.
type Footer struct {
	Rect   city.Rect
	KeyRow city.Rect
	Size   Size
	Lines  []Text
	Keys   []PlacedKey
	// Waiting is the needs-you line, which a click on WaitingRect
	// attaches; empty when nothing waits. Item 91.
	Waiting     Text
	WaitingRect city.Rect
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
			right = f.Keys[i+1].Chip.Min.X
		}
		if at.X >= k.Chip.Min.X && at.X < right {
			return k.Key, true
		}
	}
	return Key{}, false
}

// LayoutFooter lays the footer along the bottom of a width×height window.
func LayoutFooter(th Theme, width, height float64, status string, keys []Key, measure Measure) Footer {
	grid := th.Grid()
	f := Footer{Size: Small}
	_, lineH := measure("", Small)
	f.KeyRow = city.RectAt(0, height-lineH-2*grid, width, lineH+2*grid)
	f.Rect = f.KeyRow
	if status != "" {
		lines := format.Wrap(status, int((width-4*grid)/avgChar(Small, measure)))
		if len(lines) > maxFooterLines {
			lines = lines[:maxFooterLines]
		}
		top := f.KeyRow.Min.Y - lineH*float64(len(lines)) - grid
		f.Rect = city.RectAt(0, top, width, height-top)
		for i, line := range lines {
			f.Lines = append(f.Lines, Text{Text: line, At: city.Point{X: 2 * grid, Y: top + grid + lineH*float64(i)}, Size: Small})
		}
	}
	x := 2 * grid
	for _, k := range keys {
		kw, _ := measure(k.Key, Small)
		lw, _ := measure(k.Action, Small)
		chip := city.RectAt(x, f.KeyRow.Min.Y+grid/2, kw+grid, lineH+grid)
		labelX := chip.Max.X + grid
		if labelX+lw > width-2*grid {
			break
		}
		f.Keys = append(f.Keys, PlacedKey{
			Key:     k,
			Chip:    chip,
			KeyAt:   city.Point{X: x + grid/2, Y: f.KeyRow.Min.Y + grid},
			LabelAt: city.Point{X: labelX, Y: f.KeyRow.Min.Y + grid},
		})
		x = labelX + lw + 3*grid
	}
	return f
}

// WithWaiting puts the needs-you line in the slot just above the key
// row, cut to the window's width, and lifts any status above it, so the
// two never draw over each other. An empty line leaves the footer alone.
// Item 91.
func (f Footer) WithWaiting(th Theme, width float64, line string, measure Measure) Footer {
	if line == "" {
		return f
	}
	grid := th.Grid()
	_, lineH := measure("", f.Size)
	room := width - 4*grid
	for w, _ := measure(line, f.Size); w > room; w, _ = measure(line, f.Size) {
		r := []rune(strings.TrimSuffix(line, "…"))
		if len(r) <= 1 {
			line = "…"
			break
		}
		line = strings.TrimRight(string(r[:len(r)-1]), " ") + "…"
	}
	// The line is a button: a plate a line and a grid tall, its label
	// inset as a card button's is, so it reads as something to click
	// rather than a status message.
	plateH := lineH + grid
	lines := make([]Text, len(f.Lines))
	for i, l := range f.Lines {
		l.At.Y -= plateH
		lines[i] = l
	}
	f.Lines = lines
	w, _ := measure(line, f.Size)
	f.WaitingRect = city.RectAt(grid, f.KeyRow.Min.Y-plateH, w+2*grid, plateH)
	f.Waiting = Text{Text: line, At: city.Point{X: f.WaitingRect.Min.X + grid, Y: f.WaitingRect.Min.Y + grid/2}, Size: f.Size}
	top := min(f.Rect.Min.Y-plateH, f.WaitingRect.Min.Y-grid/2)
	f.Rect = city.RectAt(0, top, f.Rect.Width(), f.Rect.Max.Y-top)
	return f
}

// avgChar is the mean advance at a size, for wrapping by character count.
func avgChar(size Size, measure Measure) float64 {
	const sample = "abcdefghijklmnopqrstuvwxyz0123456789 ~/$%.|"
	w, _ := measure(sample, size)
	if w <= 0 {
		return 1
	}
	return w / float64(len(sample))
}
