package ui

import (
	"math"
	"strings"

	"botropolis.city/botropolis/pkg/city"
)

// Card is a hover or selection card laid out on a panel: a title at the
// title size and lines at the body size.
type Card struct {
	Rect     city.Rect
	Title    Text
	Kind     Text
	Move     Text
	Group    Text
	Sentence []Text
	Numbers  []Text
	Details  []Text
	Hint     Text
	Lines    []Text
	Para     []Text
	Spark    Sparkline
	Buttons  []Button
}

// LayoutCard sizes a card to its content and parks it in the top-right
// corner of bounds, a margin in; lines too wide are clipped and lines
// that would run past the bottom are dropped.
//
// A card with a face (item 88) reads from the top: what it is, small;
// the title, with whose move it is at the right of its row; the group,
// small; the sentence at body size; at most three numbers on one row;
// the buttons; and, folded out, the details, small.
func LayoutCard(th Theme, card city.Card, bounds city.Rect, measure Measure) Card {
	grid := th.Grid()
	pad, margin := 2*grid, 2*grid
	maxWidth := bounds.Width() - 2*margin - 2*pad
	_, lineH := measure("", Body)
	_, smallH := measure("", Small)
	var c Card
	width := 0.0
	grow := func(w float64) { width = max(width, w) }
	small := func(s string, y float64) Text {
		s = clipTo(s, maxWidth, Small, measure)
		w, _ := measure(s, Small)
		grow(w)
		return Text{Text: s, At: city.Point{X: pad, Y: y}, Size: Small}
	}
	y := pad
	if card.Kind != "" {
		c.Kind = small(card.Kind, y)
		y += smallH
	}
	moveW := 0.0
	room := maxWidth
	if card.Move != "" {
		moveW, _ = measure(card.Move, Small)
		room -= moveW + 2*grid
	}
	title := clipTo(card.Title, room, Title, measure)
	titleW, titleH := measure(title, Title)
	grow(titleW)
	if moveW > 0 {
		grow(titleW + 2*grid + moveW)
	}
	c.Title = Text{Text: title, At: city.Point{X: pad, Y: y}, Size: Title}
	y += titleH
	if card.Group != "" {
		c.Group = small(card.Group, y)
		y += smallH
	}
	maxLines := int((bounds.Height() - 2*margin - pad - y) / lineH)
	for _, line := range card.Lines {
		if len(c.Lines) >= maxLines {
			break
		}
		line = clipTo(line, maxWidth, Body, measure)
		w, _ := measure(line, Body)
		grow(w)
		c.Lines = append(c.Lines, Text{Text: line, At: city.Point{X: pad, Y: y}, Size: Body})
		y += lineH
	}
	if card.Sentence != "" {
		y += grid
		rows := int((bounds.Height()-2*margin-pad-y)/lineH) - 1
		for _, line := range wrapPara(card.Sentence, min(maxWidth, ParaWidth(th)), rows, measure) {
			w, _ := measure(line, Body)
			grow(w)
			c.Sentence = append(c.Sentence, Text{Text: line, At: city.Point{X: pad, Y: y}, Size: Body})
			y += lineH
		}
	}
	if len(card.Numbers) > 0 {
		y += grid
		x := pad
		for i, n := range card.Numbers {
			if i == city.FaceNumbers {
				break
			}
			label := n.Label + " " + n.Value
			w, _ := measure(label, Body)
			c.Numbers = append(c.Numbers, Text{Text: label, At: city.Point{X: x, Y: y}, Size: Body})
			grow(x + w - pad)
			x += w + 3*grid
		}
		y += lineH
	}
	if card.Para != "" {
		rows := maxLines - len(c.Lines) - 1
		top := y + lineH
		for i, line := range wrapPara(card.Para, min(maxWidth, ParaWidth(th)), rows, measure) {
			w, _ := measure(line, Body)
			grow(w)
			c.Para = append(c.Para, Text{Text: line, At: city.Point{X: pad, Y: top + lineH*float64(i)}, Size: Body})
		}
		if len(c.Para) > 0 {
			y = top + lineH*float64(len(c.Para))
		}
	}
	c.Rect = city.RectAt(0, 0, width+2*pad, y+pad)
	if len(card.Series) > 0 {
		box := city.RectAt(pad, c.Rect.Max.Y-pad+grid, c.Rect.Width()-2*pad, sparkGrids*grid)
		c.Spark = LayoutSparkline(th, card.Series, box)
		c.Rect.Max.Y = box.Max.Y + pad
	}
	if len(card.Actions) > 0 {
		row := LayoutButtonRows(th, card.Actions, city.Point{X: pad, Y: c.Rect.Max.Y - pad + grid}, maxWidth, measure)
		for _, b := range row {
			if b.Rect.Max.Y+pad > c.Rect.Max.Y {
				c.Rect.Max.Y = b.Rect.Max.Y + pad
			}
			if b.Rect.Max.X+pad > c.Rect.Max.X {
				c.Rect.Max.X = b.Rect.Max.X + pad
			}
		}
		c.Buttons = row
	}
	if card.Hint != "" && len(c.Buttons) > 0 {
		c.Hint = small(card.Hint, c.Rect.Max.Y-pad+grid/2)
		c.Rect.Max.Y = c.Hint.At.Y + smallH + pad
		c.Rect.Max.X = max(c.Rect.Max.X, width+2*pad)
	}
	if card.Open && len(card.Details) > 0 {
		y := c.Rect.Max.Y - pad + grid
		for _, line := range card.Details {
			if y+smallH+pad > bounds.Height()-2*margin {
				break
			}
			d := small(line, y)
			c.Details = append(c.Details, d)
			y += smallH
		}
		c.Rect.Max.X = max(c.Rect.Max.X, width+2*pad)
		c.Rect.Max.Y = y + pad
	}
	if moveW > 0 {
		c.Move = Text{Text: card.Move, At: city.Point{X: c.Rect.Max.X - pad - moveW, Y: c.Title.At.Y}, Size: Small}
	}
	x := bounds.Max.X - margin - c.Rect.Width()
	if x < bounds.Min.X+margin {
		x = bounds.Min.X + margin
	}
	return c.MoveTo(city.Point{X: x, Y: bounds.Min.Y + margin})
}

// PinTo places the card beside a screen rect: to its right when there is
// room, else to its left, kept inside bounds.
func (c Card) PinTo(beside city.Rect, bounds city.Rect, gap float64) Card {
	w, h := c.Rect.Width(), c.Rect.Height()
	clampY := func(y float64) float64 {
		return math.Min(math.Max(y, bounds.Min.Y), math.Max(bounds.Min.Y, bounds.Max.Y-h))
	}
	clampX := func(x float64) float64 {
		return math.Min(math.Max(x, bounds.Min.X), math.Max(bounds.Min.X, bounds.Max.X-w))
	}
	// Beside first, on whichever side has room, then under, then over.
	// The card must not land on the building it describes: a click
	// selects now rather than attaching, so the card it raises carries
	// stop two buttons from the left, and a second click out of
	// double-click habit would otherwise press it. Kept off the
	// building, the worst a second click can do is select the same
	// building again.
	for _, at := range []city.Point{
		{X: beside.Max.X + gap, Y: clampY(beside.Min.Y)},
		{X: beside.Min.X - gap - w, Y: clampY(beside.Min.Y)},
		{X: clampX(beside.Min.X), Y: beside.Max.Y + gap},
		{X: clampX(beside.Min.X), Y: beside.Min.Y - gap - h},
	} {
		if at.X < bounds.Min.X || at.X+w > bounds.Max.X {
			continue
		}
		if at.Y < bounds.Min.Y || at.Y+h > bounds.Max.Y {
			continue
		}
		return c.MoveTo(at)
	}
	// Nowhere clear of it fits on screen. Stay in the window and accept
	// the overlap; a window this small has worse problems.
	return c.MoveTo(city.Point{X: clampX(beside.Max.X + gap), Y: clampY(beside.Min.Y)})
}

// MoveTo puts the card's top-left corner at a point, carrying its text.
func (c Card) MoveTo(at city.Point) Card {
	by := at.Sub(c.Rect.Min)
	moved := Card{Rect: city.Rect{Min: at, Max: c.Rect.Max.Add(by)}, Title: c.Title.moved(by),
		Kind: c.Kind.moved(by), Move: c.Move.moved(by), Group: c.Group.moved(by), Hint: c.Hint.moved(by)}
	for _, line := range c.Lines {
		moved.Lines = append(moved.Lines, line.moved(by))
	}
	for _, line := range c.Sentence {
		moved.Sentence = append(moved.Sentence, line.moved(by))
	}
	for _, line := range c.Numbers {
		moved.Numbers = append(moved.Numbers, line.moved(by))
	}
	for _, line := range c.Details {
		moved.Details = append(moved.Details, line.moved(by))
	}
	for _, line := range c.Para {
		moved.Para = append(moved.Para, line.moved(by))
	}
	for _, b := range c.Buttons {
		moved.Buttons = append(moved.Buttons, Button{Rect: city.Rect{Min: b.Rect.Min.Add(by), Max: b.Rect.Max.Add(by)}, Label: b.Label.moved(by)})
	}
	if len(c.Spark.Points) > 0 {
		moved.Spark = Sparkline{Box: city.Rect{Min: c.Spark.Box.Min.Add(by), Max: c.Spark.Box.Max.Add(by)}, Last: c.Spark.Last.Add(by), Peak: c.Spark.Peak}
		for _, p := range c.Spark.Points {
			moved.Spark.Points = append(moved.Spark.Points, p.Add(by))
		}
	}
	return moved
}

// ParaWidth is the measure a card's paragraph wraps to: wide enough to
// read as prose, narrow enough that a line is one glance.
func ParaWidth(th Theme) float64 { return 60 * th.Grid() }

// wrapPara breaks text into lines no wider than width at word
// boundaries, keeping its own line breaks. A word wider than the
// measure is clipped; past rows lines the last kept one ends in an
// ellipsis.
func wrapPara(text string, width float64, rows int, measure Measure) []string {
	if rows < 1 {
		return nil
	}
	var lines []string
	for _, para := range strings.Split(text, "\n") {
		line := ""
		for _, word := range strings.Fields(para) {
			next := word
			if line != "" {
				next = line + " " + word
			}
			if w, _ := measure(next, Body); w <= width || line == "" {
				line = clipTo(next, width, Body, measure)
				continue
			}
			lines = append(lines, line)
			line = clipTo(word, width, Body, measure)
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) > rows {
		lines = lines[:rows]
		ellipsis, _ := measure("…", Body)
		lines[rows-1] = clipTo(lines[rows-1], width-ellipsis, Body, measure) + "…"
	}
	return lines
}
