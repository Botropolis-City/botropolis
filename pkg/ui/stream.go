package ui

import (
	"slices"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/events"
)

// The stream (item 88): the event log's live tail as a column down the
// right, beside the map, newest first. Routine entries are muted, so
// what needs you or went wrong is the only thing that stands out.

// StreamGrids is the column's width in grid units.
const StreamGrids = 40

// StreamEmpty is what the column says before anything has happened.
const StreamEmpty = "nothing has happened since the city opened"

// StreamRow is one entry: its time and what happened, on a band that
// takes a click.
type StreamRow struct {
	Event events.Event
	Rect  city.Rect
	Time  Text
	Text  Text
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
func StreamArea(th Theme, width, height, top, footer float64) city.Rect {
	w := StreamGrids * th.Grid()
	bottom := height - footer
	if mini, ok := LayoutMinimap(th, width, height, footer); ok {
		bottom = mini.Min.Y - th.Grid()
	}
	return city.RectAt(width-w, top, w, bottom-top)
}

// LayoutStream lays the events in the area, newest first, as many as
// fit.
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
	rowH := lineH + grid
	for _, e := range ordered {
		if y+rowH > area.Max.Y-pad {
			break
		}
		text := e.Title
		if text == "" {
			text = e.SessionID
		}
		text = clipTo(string(e.Kind)+" · "+text, inner, Small, measure)
		x := area.Min.X + pad
		s.Rows = append(s.Rows, StreamRow{
			Event: e,
			Rect:  city.RectAt(area.Min.X+grid, y-grid/2, area.Width()-2*grid, rowH),
			Time:  Text{Text: e.At.Local().Format("15:04"), At: city.Point{X: x, Y: y}, Size: Small},
			Text:  Text{Text: text, At: city.Point{X: x + timeW + grid, Y: y}, Size: Small},
			Muted: !city.Flagged(e.Kind),
		})
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
