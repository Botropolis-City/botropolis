package ui

import (
	"slices"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/events"
)

// The stream (item 88): the event log's live tail as a column down the
// right, beside the map, newest first. Routine entries are muted, so
// what needs you or went wrong is the only thing that stands out.

// StreamGrids is the column's full width in grid units, which it takes
// when a quarter of the window is at least that (item 100).
const StreamGrids = 40

// StreamMinGrids is the narrowest the column is shown by itself; in a
// window whose quarter is narrower, it hides until its key shows it.
const StreamMinGrids = 24

// streamRows is the most lines one entry wraps onto.
const streamRows = 6

// StreamEmpty is what the column says before anything has happened.
const StreamEmpty = "nothing has happened since the city opened"

// StreamView is whether the column is up: Hidden is the remembered
// hide, and Asked a show its key made for this run in a window too
// narrow for it, which never touches that hide.
type StreamView struct{ Hidden, Asked bool }

// Shown is whether the column is up in a window it does or does not
// fit.
func (v StreamView) Shown(fits bool) bool {
	if fits {
		return !v.Hidden
	}
	return v.Asked
}

// Toggle is what the column's key does.
func (v StreamView) Toggle(fits bool) StreamView {
	if fits {
		v.Hidden = !v.Hidden
	} else {
		v.Asked = !v.Asked
	}
	return v
}

// StreamFits is whether a quarter of the window holds the column at its
// minimum.
func StreamFits(th Theme, width float64) bool {
	return width/4 >= StreamMinGrids*th.Grid()
}

// StreamWidth is the column's width: a quarter of the window up to its
// full width, or its minimum, within the window, when that does not
// fit.
func StreamWidth(th Theme, width float64) float64 {
	if StreamFits(th, width) {
		return min(width/4, StreamGrids*th.Grid())
	}
	return min(StreamMinGrids*th.Grid(), width)
}

// StreamRow is one entry: its time and what happened, wrapped to the
// column, on a band that takes a click.
type StreamRow struct {
	Event events.Event
	Rect  city.Rect
	Time  Text
	Lines []Text
	Muted bool
}

// Stream is the column laid out.
type Stream struct {
	Rect  city.Rect
	Title Text
	Rows  []StreamRow
	Empty Text
}

// StreamArea is the column's room: down the right edge, under the
// strip, and stopping above the minimap.
// Corner is which right-hand corner the stream's panel takes.
type Corner int

const (
	// TopRight is under the strip.
	TopRight Corner = iota
	// BottomRight is above the minimap, or the footer without one.
	BottomRight
)

func (c Corner) String() string { return [...]string{"top right", "bottom right"}[c] }

// StreamCorner is where the panel sits; Aria picks between the two from
// frames of each (item 100).
const StreamCorner = TopRight

// StreamPanelGrids is the panel's height in grid units, about a card's,
// so it is a corner panel rather than a column (item 100).
const StreamPanelGrids = 24

// StreamArea is the panel's room: against the right edge, in its
// corner, between the strip and the minimap or footer.
func StreamArea(th Theme, corner Corner, width, height, top, footer float64) city.Rect {
	w := StreamWidth(th, width)
	bottom := height - footer
	if mini, ok := LayoutMinimap(th, width, height, footer); ok && mini.Min.Y-th.Grid() > top {
		bottom = mini.Min.Y - th.Grid()
	}
	h := min(StreamPanelGrids*th.Grid(), max(bottom-top, 0))
	if corner == BottomRight {
		return city.RectAt(width-w, bottom-h, w, h)
	}
	return city.RectAt(width-w, top, w, h)
}

// LayoutStream lays the events in the area, newest first, as many as
// fit, each wrapped to the column.
func LayoutStream(th Theme, list []events.Event, area city.Rect, measure Measure) Stream {
	grid := th.Grid()
	pad := 2 * grid
	_, titleH := measure("", Body)
	_, lineH := measure("", Small)
	s := Stream{Rect: area}
	s.Title = Text{Text: "Stream", At: area.Min.Add(city.Point{X: pad, Y: pad}), Size: Body}
	ordered := slices.Clone(list)
	slices.SortStableFunc(ordered, func(a, b events.Event) int { return b.At.Compare(a.At) })
	timeW, _ := measure("00:00", Small)
	inner := area.Width() - 2*pad - timeW - grid
	y := area.Min.Y + pad + titleH + grid
	if len(ordered) == 0 {
		s.Empty = Text{Text: StreamEmpty, At: city.Point{X: area.Min.X + pad, Y: y}, Size: Small}
	}
	for _, e := range ordered {
		text := e.Title
		if text == "" {
			text = e.SessionID
		}
		wrapped := wrapAt(string(e.Kind)+" · "+text, inner, streamRows, Small, measure)
		rowH := float64(len(wrapped))*lineH + grid
		if y+rowH > area.Max.Y-pad {
			break
		}
		x := area.Min.X + pad
		row := StreamRow{
			Event: e,
			Rect:  city.RectAt(area.Min.X+grid, y-grid/2, area.Width()-2*grid, rowH),
			Time:  Text{Text: e.At.Local().Format("15:04"), At: city.Point{X: x, Y: y}, Size: Small},
			Muted: !city.Flagged(e.Kind),
		}
		for i, line := range wrapped {
			row.Lines = append(row.Lines, Text{Text: line, At: city.Point{X: x + timeW + grid, Y: y + float64(i)*lineH}, Size: Small})
		}
		s.Rows = append(s.Rows, row)
		y += rowH
	}
	return s
}

// Hit is the event a click lands on.
func (s Stream) Hit(at city.Point) (events.Event, bool) {
	for _, r := range s.Rows {
		if r.Rect.Contains(at) {
			return r.Event, true
		}
	}
	return events.Event{}, false
}
