package ui

import (
	"math"
	"strings"

	"botropolis.city/botropolis/pkg/city"
)

// Card is a hover or selection card laid out on a panel: a title at the
// title size and lines at the body size.
type Card struct {
	Rect    city.Rect
	Title   Text
	Lines   []Text
	Para    []Text
	Spark   Sparkline
	Buttons []Button
}

// LayoutCard sizes a card to its content and parks it in the top-right
// corner of bounds, a margin in; lines too wide are clipped and lines
// that would run past the bottom are dropped.
func LayoutCard(th Theme, card city.Card, bounds city.Rect, measure Measure) Card {
	pad, margin := 2*th.Grid(), 2*th.Grid()
	maxWidth := bounds.Width() - 2*margin - 2*pad
	title := clipTo(card.Title, maxWidth, Title, measure)
	width, titleH := measure(title, Title)
	_, lineH := measure("", Body)
	maxLines := int((bounds.Height() - 2*margin - 2*pad - titleH) / lineH)
	var lines []string
	for _, line := range card.Lines {
		if len(lines) >= maxLines {
			break
		}
		line = clipTo(line, maxWidth, Body, measure)
		if w, _ := measure(line, Body); w > width {
			width = w
		}
		lines = append(lines, line)
	}
	c := Card{Rect: city.RectAt(0, 0, width+2*pad, 2*pad+titleH+lineH*float64(len(lines)))}
	c.Title = Text{Text: title, At: city.Point{X: pad, Y: pad}, Size: Title}
	for i, line := range lines {
		c.Lines = append(c.Lines, Text{Text: line, At: city.Point{X: pad, Y: pad + titleH + lineH*float64(i)}, Size: Body})
	}
	if card.Para != "" {
		rows := maxLines - len(lines) - 1
		para := wrapPara(card.Para, min(maxWidth, ParaWidth(th)), rows, measure)
		top := pad + titleH + lineH*float64(len(lines)+1)
		for i, line := range para {
			if w, _ := measure(line, Body); w > width {
				width = w
			}
			c.Para = append(c.Para, Text{Text: line, At: city.Point{X: pad, Y: top + lineH*float64(i)}, Size: Body})
		}
		if len(para) > 0 {
			c.Rect.Max.X = width + 2*pad
			c.Rect.Max.Y = top + lineH*float64(len(para)) + pad
		}
	}
	if len(card.Series) > 0 {
		box := city.RectAt(pad, c.Rect.Max.Y-pad+th.Grid(), c.Rect.Width()-2*pad, sparkGrids*th.Grid())
		c.Spark = LayoutSparkline(th, card.Series, box)
		c.Rect.Max.Y = box.Max.Y + pad
	}
	if len(card.Actions) > 0 {
		row := LayoutButtonRows(th, card.Actions, city.Point{X: pad, Y: c.Rect.Max.Y - pad + th.Grid()}, maxWidth, measure)
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
	moved := Card{Rect: city.Rect{Min: at, Max: c.Rect.Max.Add(by)}, Title: c.Title.moved(by)}
	for _, line := range c.Lines {
		moved.Lines = append(moved.Lines, line.moved(by))
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
