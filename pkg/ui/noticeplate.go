package ui

import "botropolis.city/botropolis/pkg/city"

// noticeLines is the most lines a notice wraps onto.
const noticeLines = 3

// NoticePlate is a notice laid out: one plate centred under the strip,
// the tone's dot before the first line, the text in Body type, and the
// key that acts on it as a cap after the last line (item 352). Dot and
// Cap are empty when the message has no tone or no key.
type NoticePlate struct {
	Rect  city.Rect
	Dot   city.Rect
	Tone  Tone
	Lines []Text
	Cap   KeyCap
}

// LayoutNotice lays a message out across a window of width, under the
// chrome that ends at top; an empty message lays out nothing.
func LayoutNotice(th Theme, m Message, width, top float64, measure Measure) NoticePlate {
	if m.Text == "" {
		return NoticePlate{}
	}
	grid := th.Grid()
	padX, padY := 2*grid, grid
	_, lineH := measure("", Body)
	dotW, capW := 0.0, 0.0
	if m.Tone != ToneNone {
		dotW = 2 * grid
	}
	var cap KeyCap
	if m.Key != "" {
		cap = LayoutKeyCap(th, m.Key, city.Point{}, measure)
		capW = cap.Rect.Width() + grid
	}
	room := width - 4*grid - 2*padX - dotW - capW
	lines := wrapAt(m.Text, room, noticeLines, Body, measure)
	textW := 0.0
	for _, l := range lines {
		w, _ := measure(l, Body)
		textW = max(textW, w)
	}
	w := 2*padX + dotW + textW + capW
	h := 2*padY + lineH*float64(len(lines))
	p := NoticePlate{Rect: city.RectAt((width-w)/2, top+grid, w, h), Tone: m.Tone}
	x, y := p.Rect.Min.X+padX, p.Rect.Min.Y+padY
	if m.Tone != ToneNone {
		p.Dot = city.RectAt(x, y+(lineH-grid)/2, grid, grid)
		x += dotW
	}
	for i, l := range lines {
		p.Lines = append(p.Lines, Text{Text: l, At: city.Point{X: x, Y: y + lineH*float64(i)}, Size: Body})
	}
	if m.Key != "" {
		last := p.Lines[len(p.Lines)-1]
		lastW, _ := measure(last.Text, Body)
		p.Cap = cap.Moved(city.Point{X: last.At.X + lastW + grid, Y: last.At.Y + (lineH-cap.Rect.Height())/2})
	}
	return p
}
