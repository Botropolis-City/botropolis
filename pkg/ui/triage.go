package ui

import (
	"fmt"

	"botropolis.city/botropolis/pkg/city"
)

// TriageKeys are triage's own keys, a mode of their own (item 88).
var TriageKeys = []Key{
	{Key: "enter", Action: "act"},
	{Key: "p", Action: "rest"},
	{Key: "d", Action: "pack up"},
	{Key: "s", Action: "snooze"},
	{Key: "n", Action: "next"},
	{Key: "esc", Action: "leave triage"},
}

// LayoutTriageBar is the footer while triage walks: which step of how
// many and the session's name, over triage's keys.
func LayoutTriageBar(th Theme, width, height float64, index, count int, title string, measure Measure) Footer {
	status := fmt.Sprintf("triage %d of %d · %s", index+1, count, title)
	return LayoutFooter(th, width, height, status, TriageKeys, measure)
}

// SnoozePicker is the snooze's choices: the ready times, then a line to
// type one, with why a typed time was refused under it.
type SnoozePicker struct {
	Rect   city.Rect
	Title  Text
	Rows   []Text
	Bands  []city.Rect
	Cursor int
	Error  Text
}

// SnoozePrompt is the typed line's prompt.
const SnoozePrompt = "or type a time: "

// LayoutSnoozePicker centres the picker in the window.
func LayoutSnoozePicker(th Theme, width, height float64, choices []string, cursor int, typed, refused string, measure Measure) SnoozePicker {
	grid := th.Grid()
	pad := 3 * grid
	_, titleH := measure("", Title)
	_, bodyH := measure("", Body)
	_, smallH := measure("", Small)
	rows := append(append([]string{}, choices...), SnoozePrompt+typed+"▏")
	w := 0.0
	for _, r := range append(rows, refused) {
		rw, _ := measure(r, Body)
		w = max(w, rw)
	}
	w = max(w+2*pad, 40*grid)
	rowH := bodyH + grid
	h := pad + titleH + grid + rowH*float64(len(rows)) + smallH + pad
	p := SnoozePicker{Rect: city.RectAt((width-w)/2, (height-h)/2, w, h), Cursor: cursor}
	x, y := p.Rect.Min.X+pad, p.Rect.Min.Y+pad
	p.Title = Text{Text: "Snooze until", At: city.Point{X: x, Y: y}, Size: Title}
	y += titleH + grid
	for _, r := range rows {
		p.Bands = append(p.Bands, city.RectAt(p.Rect.Min.X+grid, y-grid/2, w-2*grid, rowH))
		p.Rows = append(p.Rows, Text{Text: r, At: city.Point{X: x, Y: y}, Size: Body})
		y += rowH
	}
	p.Error = Text{Text: refused, At: city.Point{X: x, Y: y}, Size: Small}
	return p
}
