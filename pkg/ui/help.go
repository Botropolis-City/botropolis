package ui

import (
	"fmt"
	"math"

	"botropolis.city/botropolis/pkg/city"
)

// Help is the overlay listing every key: a titled panel of key chips
// with their actions, in as few columns as fit the window, or in one
// column that scrolls by whole rows when none do. Item 127.
type Help struct {
	Rect    city.Rect
	Title   Text
	Rows    []HelpRow
	Columns int
	// Skip is how many rows are scrolled past, and Max the most it can
	// be: the last row at the foot of the panel.
	Skip, Max int
	// Above and Below say how many rows are out of view either side;
	// empty when none are.
	Above, Below Text
}

// HelpRow is one key: its chip, and its action wrapped to the column,
// on a band that takes a click.
type HelpRow struct {
	Key   Key
	Band  city.Rect
	Chip  city.Rect
	KeyAt city.Point
	Label []Text
}

// Hit finds the key under a screen point. A row's target is its whole
// band across its column, chip and action together, so the thing you
// read is the thing you can press. Bug 43.
func (h Help) Hit(at city.Point) (Key, bool) {
	for _, r := range h.Rows {
		if r.Band.Contains(at) {
			return r.Key, true
		}
	}
	return Key{}, false
}

// Moved is the list shifted by a distance, so it can be laid out in the
// room between the strip and the footer and then put there.
func (h Help) Moved(by city.Point) Help {
	shift := func(r city.Rect) city.Rect { return city.Rect{Min: r.Min.Add(by), Max: r.Max.Add(by)} }
	h.Rect = shift(h.Rect)
	h.Title, h.Above, h.Below = h.Title.moved(by), h.Above.moved(by), h.Below.moved(by)
	rows := make([]HelpRow, len(h.Rows))
	for i, r := range h.Rows {
		r.Band, r.Chip, r.KeyAt = shift(r.Band), shift(r.Chip), r.KeyAt.Add(by)
		label := make([]Text, len(r.Label))
		for j, l := range r.Label {
			label[j] = l.moved(by)
		}
		r.Label = label
		rows[i] = r
	}
	h.Rows = rows
	return h
}

// helpMetrics is what every arrangement of the list shares.
type helpMetrics struct {
	grid, pad, gap, lineH, titleH, chipW, chipH, labelW, titleW float64
	measure                                                     Measure
}

// helpRow is a row measured for one label width, before it is placed.
type helpRow struct {
	key   Key
	lines []string
	h     float64
}

func (m helpMetrics) rows(keys []Key, labelW float64) []helpRow {
	out := make([]helpRow, len(keys))
	for i, k := range keys {
		lines := wrapAt(k.Action, labelW, math.MaxInt, Small, m.measure)
		if len(lines) == 0 {
			lines = []string{""}
		}
		out[i] = helpRow{key: k, lines: lines, h: m.chipH + float64(len(lines)-1)*m.lineH}
	}
	return out
}

// columnWidth is the widest a column's chip and label come to.
func (m helpMetrics) columnWidth(rows []helpRow) float64 {
	var w float64
	for _, r := range rows {
		for _, l := range r.lines {
			if lw, _ := m.measure(l, Small); lw > w {
				w = lw
			}
		}
	}
	return m.chipW + m.grid + w
}

// split breaks the rows, in order, into at most n columns no taller
// than limit, and reports false when they need more.
func (m helpMetrics) split(rows []helpRow, n int, limit float64) ([][]helpRow, bool) {
	var cols [][]helpRow
	var h float64
	for _, r := range rows {
		if len(cols) == 0 || h+m.grid/2+r.h > limit {
			cols = append(cols, []helpRow{r})
			h = r.h
		} else {
			cols[len(cols)-1] = append(cols[len(cols)-1], r)
			h += m.grid/2 + r.h
		}
		if h > limit {
			return nil, false
		}
	}
	return cols, len(cols) <= n
}

func (m helpMetrics) height(col []helpRow) float64 {
	var h float64
	for i, r := range col {
		if i > 0 {
			h += m.grid / 2
		}
		h += r.h
	}
	return h
}

// LayoutHelp lays the key list out in a width×height window, centred.
// It takes the fewest columns, read down and then across, in which every
// key fits; when no number does, it shows one column scrolled skip rows
// down, with a mark for what is above and below, and places only the
// rows that fit whole. Item 127.
func LayoutHelp(th Theme, width, height float64, title string, keys []Key, measure Measure, skip int) Help {
	grid := th.Grid()
	m := helpMetrics{grid: grid, pad: 3 * grid, gap: 3 * grid, measure: measure}
	_, m.lineH = measure("", Small)
	m.titleW, m.titleH = measure(title, Title)
	m.chipH = m.lineH + grid
	for _, k := range keys {
		if w, _ := measure(k.Key, Small); w+grid > m.chipW {
			m.chipW = w + grid
		}
		if w, _ := measure(k.Action, Small); w > m.labelW {
			m.labelW = w
		}
	}
	margin := 2 * grid
	roomW := width - 2*margin - 2*m.pad
	roomH := height - 2*margin - 2*m.pad - m.titleH - grid
	minLabel := 12 * m.lineH

	for n := 1; n <= len(keys); n++ {
		labelW := min(m.labelW, (roomW-float64(n-1)*m.gap)/float64(n)-m.chipW-m.grid)
		if n > 1 && labelW < minLabel {
			break
		}
		rows := m.rows(keys, max(labelW, 1))
		if m.height(rows) <= roomH {
			return m.columns(width, height, title, [][]helpRow{rows})
		}
		if n == 1 {
			continue
		}
		if cols, ok := m.balanced(rows, n, roomH); ok {
			return m.columns(width, height, title, cols)
		}
	}
	labelW := max(min(m.labelW, roomW-m.chipW-m.grid), 1)
	return m.scrolled(width, height, title, m.rows(keys, labelW), roomH, skip)
}

// balanced finds the shortest columns n of them can hold the rows in,
// when that is no taller than room.
func (m helpMetrics) balanced(rows []helpRow, n int, room float64) ([][]helpRow, bool) {
	best, ok := m.split(rows, n, room)
	if !ok {
		return nil, false
	}
	lo, hi := 0.0, room
	for hi-lo > 0.5 {
		mid := (lo + hi) / 2
		if cols, fits := m.split(rows, n, mid); fits {
			best, hi = cols, mid
		} else {
			lo = mid
		}
	}
	return best, true
}

// chipsOf is the metrics with the chip column as wide as these rows'
// widest chip, so a column's labels start just past its own chips.
func (m helpMetrics) chipsOf(rows []helpRow) helpMetrics {
	m.chipW = 0
	for _, r := range rows {
		if w, _ := m.measure(r.key.Key, Small); w+m.grid > m.chipW {
			m.chipW = w + m.grid
		}
	}
	return m
}

// columns places rows that all fit, in their columns.
func (m helpMetrics) columns(width, height float64, title string, cols [][]helpRow) Help {
	var w, h float64
	widths := make([]float64, len(cols))
	each := make([]helpMetrics, len(cols))
	for i, c := range cols {
		each[i] = m.chipsOf(c)
		widths[i] = each[i].columnWidth(c)
		w += widths[i]
		h = max(h, m.height(c))
	}
	w += float64(len(cols)-1) * m.gap
	w = max(w, m.titleW)
	pw, ph := m.pad+w+m.pad, m.pad+m.titleH+m.grid+h+m.pad
	help := Help{Columns: len(cols), Rect: city.RectAt((width-pw)/2, (height-ph)/2, pw, ph)}
	help.Title = Text{Text: title, At: help.Rect.Min.Add(city.Point{X: m.pad, Y: m.pad}), Size: Title}
	x := help.Rect.Min.X + m.pad
	top := help.Rect.Min.Y + m.pad + m.titleH + m.grid
	for i, c := range cols {
		y := top
		for _, r := range c {
			help.Rows = append(help.Rows, each[i].place(r, x, y, widths[i]))
			y += r.h + m.grid/2
		}
		x += widths[i] + m.gap
	}
	return help
}

// scrolled places one column of rows, skip rows down, in as much of the
// window's height as there is.
func (m helpMetrics) scrolled(width, height float64, title string, rows []helpRow, room float64, skip int) Help {
	markH := m.lineH + m.grid/2
	// fits counts how many rows from first fit in h.
	fits := func(first int, h float64) int {
		used, n := 0.0, 0
		for _, r := range rows[first:] {
			step := r.h
			if n > 0 {
				step += m.grid / 2
			}
			if used+step > h {
				break
			}
			used += step
			n++
		}
		return n
	}
	maxSkip := len(rows) - 1
	for s := 0; s < len(rows); s++ {
		h := room
		if s > 0 {
			h -= markH
		}
		if fits(s, h) == len(rows)-s {
			maxSkip = s
			break
		}
	}
	skip = min(max(skip, 0), maxSkip)
	h := room
	if skip > 0 {
		h -= markH
	}
	shown := fits(skip, h)
	if skip+shown < len(rows) {
		shown = fits(skip, h-markH)
	}
	shown = max(shown, 1)
	colW := max(m.columnWidth(rows), m.titleW)
	pw, ph := m.pad+colW+m.pad, m.pad+m.titleH+m.grid+room+m.pad
	help := Help{Columns: 1, Skip: skip, Max: maxSkip, Rect: city.RectAt((width-pw)/2, (height-ph)/2, pw, ph)}
	help.Title = Text{Text: title, At: help.Rect.Min.Add(city.Point{X: m.pad, Y: m.pad}), Size: Title}
	x := help.Rect.Min.X + m.pad
	y := help.Rect.Min.Y + m.pad + m.titleH + m.grid
	if skip > 0 {
		help.Above = Text{Text: fmt.Sprintf("%d more above", skip), At: city.Point{X: x, Y: y}, Size: Small}
		y += markH
	}
	for _, r := range rows[skip : skip+shown] {
		help.Rows = append(help.Rows, m.place(r, x, y, colW))
		y += r.h + m.grid/2
	}
	if rest := len(rows) - skip - shown; rest > 0 {
		help.Below = Text{Text: fmt.Sprintf("%d more below", rest), At: city.Point{X: x, Y: help.Rect.Max.Y - m.pad - m.lineH}, Size: Small}
	}
	return help
}

func (m helpMetrics) place(r helpRow, x, y, colW float64) HelpRow {
	kw, _ := m.measure(r.key.Key, Small)
	chip := city.RectAt(x, y, kw+m.grid, m.chipH)
	row := HelpRow{
		Key:   r.key,
		Chip:  chip,
		Band:  city.RectAt(x, y, colW, r.h),
		KeyAt: city.Point{X: x + m.grid/2, Y: y + m.grid/2},
	}
	for i, l := range r.lines {
		row.Label = append(row.Label, Text{Text: l, At: city.Point{X: x + m.chipW + m.grid, Y: y + m.grid/2 + float64(i)*m.lineH}, Size: Small})
	}
	return row
}
