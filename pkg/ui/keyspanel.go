package ui

import "botropolis.city/botropolis/pkg/city"

// The settings' Keys panel (item 350): every rebindable key under where
// it answers, what it does and its key as a cap, scrolled to keep the
// focused row drawn.

// KeyRow is one binding as the panel shows it.
type KeyRow struct {
	Section, Label, Key string
	// Recording is whether the row waits for a key, and Refused why the
	// key last pressed for it was not taken.
	Recording bool
	Refused   string
}

// KeysPanelRow is one row as laid out, by its index in the rows given.
type KeysPanelRow struct {
	Index   int
	Row     city.Rect
	Label   Text
	Cap     KeyCap
	Note    Text
	Focused bool
}

// KeysPanel is the panel as laid out.
type KeysPanel struct {
	Rect     city.Rect
	Title    Text
	Headings []Text
	Rows     []KeysPanelRow
}

const keysTitle = "Keys"

// recordingCap is what a row's cap reads while it waits for a key.
const recordingCap = "press a key…"

// keysLine is one line of the panel: a section's heading or a row.
type keysLine struct {
	heading string
	row     int
}

// LayoutKeysPanel centres the panel in a width×height window and lays
// out as many lines as fit, starting where the focused row still shows.
func LayoutKeysPanel(th Theme, width, height float64, rows []KeyRow, focus int, measure Measure) KeysPanel {
	grid := th.Grid()
	pad := 3 * grid
	_, lineH := measure("", Body)
	_, smallH := measure("", Small)
	_, titleH := measure(keysTitle, Title)
	rowH := lineH + grid
	var lines []keysLine
	section := ""
	labelW, capW := 0.0, 0.0
	for i, r := range rows {
		if r.Section != section {
			section = r.Section
			lines = append(lines, keysLine{heading: section, row: -1})
		}
		lines = append(lines, keysLine{row: i})
		w, _ := measure(r.Label, Body)
		labelW = max(labelW, w)
		capW = max(capW, LayoutKeyCap(th, recordingCap, city.Point{}, measure).Rect.Width())
		if r.Refused != "" {
			w, _ := measure(r.Refused, Small)
			labelW = max(labelW, w)
		}
	}
	lineHeight := func(l keysLine) float64 {
		if l.row < 0 {
			return smallH + grid
		}
		if rows[l.row].Refused != "" {
			return rowH + smallH
		}
		return rowH
	}
	room := height - 2*pad - titleH - grid - 4*grid
	focusLine := 0
	for i, l := range lines {
		if l.row == focus {
			focusLine = i
		}
	}
	// Start at the top, and move down only as far as the focused row
	// needs to be drawn.
	start := 0
	for {
		used := 0.0
		end := start
		for end < len(lines) && used+lineHeight(lines[end]) <= room {
			used += lineHeight(lines[end])
			end++
		}
		if focusLine < end || start >= focusLine {
			break
		}
		start++
	}
	w := pad + labelW + 3*grid + capW + pad
	if tw, _ := measure(keysTitle, Title); pad+tw+pad > w {
		w = pad + tw + pad
	}
	used := 0.0
	end := start
	for end < len(lines) && used+lineHeight(lines[end]) <= room {
		used += lineHeight(lines[end])
		end++
	}
	h := pad + titleH + grid + used + pad
	p := KeysPanel{Rect: city.RectAt((width-w)/2, max(0, (height-h)/2), w, h)}
	p.Title = Text{Text: keysTitle, At: p.Rect.Min.Add(city.Point{X: pad, Y: pad}), Size: Title}
	y := p.Rect.Min.Y + pad + titleH + grid
	for _, l := range lines[start:end] {
		if l.row < 0 {
			p.Headings = append(p.Headings, Text{Text: l.heading, At: city.Point{X: p.Rect.Min.X + pad, Y: y}, Size: Small})
			y += lineHeight(l)
			continue
		}
		r := rows[l.row]
		key := r.Key
		if r.Recording {
			key = recordingCap
		}
		row := KeysPanelRow{
			Index:   l.row,
			Row:     city.RectAt(p.Rect.Min.X, y, w, lineHeight(l)),
			Label:   Text{Text: r.Label, At: city.Point{X: p.Rect.Min.X + pad, Y: y + grid/2}, Size: Body},
			Focused: l.row == focus,
		}
		row.Cap = LayoutKeyCap(th, key, city.Point{}, measure)
		row.Cap = row.Cap.Moved(city.Point{X: p.Rect.Max.X - pad - row.Cap.Rect.Width(), Y: y + (rowH-row.Cap.Rect.Height())/2})
		if r.Refused != "" {
			row.Note = Text{Text: r.Refused, At: city.Point{X: p.Rect.Min.X + pad, Y: y + rowH}, Size: Small}
		}
		p.Rows = append(p.Rows, row)
		y += lineHeight(l)
	}
	return p
}
