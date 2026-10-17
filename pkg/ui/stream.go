package ui

import (
	"fmt"
	"math"
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

// Stream is the panel laid out: the entries in view, and, when they do
// not all fit, how far it is scrolled, the mark for what is newer above
// and the bar down its right edge (item 100).
type Stream struct {
	Rect  city.Rect
	Title Text
	Rows  []StreamRow
	Empty Text
	// Skip is how many of the newest entries are scrolled past, and Max
	// the most it can be: the oldest entry last in view.
	Skip, Max int
	// Newer says how many entries are above the view, on a band a click
	// takes back to the newest.
	Newer     Text
	NewerRect city.Rect
	// Track and Thumb are the scroll bar, empty when everything fits.
	Track, Thumb city.Rect
}

// SkipAt is the scroll that puts the thumb's middle at y on the track.
func (s Stream) SkipAt(y float64) int {
	room := s.Track.Height() - s.Thumb.Height()
	if room <= 0 || s.Max == 0 {
		return 0
	}
	frac := min(max((y-s.Track.Min.Y-s.Thumb.Height()/2)/room, 0), 1)
	return int(math.Round(frac * float64(s.Max)))
}

// FollowStream keeps a scrolled-back view on the same entries as more
// arrive above it; at the newest, it stays at the newest.
func FollowStream(skip, before, after int) int {
	if skip == 0 {
		return 0
	}
	return max(skip+after-before, 0)
}

// MinimapBesideStream moves the minimap up out of the panel's way when
// the panel sits on the footer in the bottom right.
func MinimapBesideStream(th Theme, mini, panel city.Rect, corner Corner) city.Rect {
	if corner != BottomRight || panel.Area() == 0 || !mini.Overlaps(panel) {
		return mini
	}
	return city.RectAt(mini.Min.X, panel.Min.Y-th.Grid()-mini.Height(), mini.Width(), mini.Height())
}

// Corner is which right-hand corner the stream's panel takes.
type Corner int

const (
	// TopRight is under the strip.
	TopRight Corner = iota
	// BottomRight is on the footer.
	BottomRight
)

func (c Corner) String() string { return [...]string{"top right", "bottom right"}[c] }

// StreamCorner is where the panel sits; Aria picks between the two from
// frames of each (item 100).
const StreamCorner = TopRight

// StreamPanelGrids is the panel's height in grid units, about a card's,
// so it is a corner panel rather than a column (item 100).
const StreamPanelGrids = 24

// StreamArea is the panel's room: against the right edge, under the
// strip and above the minimap in the top right, or flush on the footer
// in the bottom right.
func StreamArea(th Theme, corner Corner, width, height, top, footer float64) city.Rect {
	w := StreamWidth(th, width)
	bottom := height - footer
	if corner == BottomRight {
		h := min(StreamPanelGrids*th.Grid(), max(bottom-top, 0))
		return city.RectAt(width-w, bottom-h, w, h)
	}
	if mini, ok := LayoutMinimap(th, width, height, footer); ok && mini.Min.Y-th.Grid() > top {
		bottom = mini.Min.Y - th.Grid()
	}
	h := min(StreamPanelGrids*th.Grid(), max(bottom-top, 0))
	return city.RectAt(width-w, top, w, h)
}

// LayoutStream lays the events in the area, newest first, wrapped to the
// panel, starting skip entries back from the newest.
func LayoutStream(th Theme, list []events.Event, area city.Rect, measure Measure, skip int) Stream {
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
	top := area.Min.Y + pad + titleH + grid
	bottom := area.Max.Y - pad
	if len(ordered) == 0 {
		s.Empty = Text{Text: StreamEmpty, At: city.Point{X: area.Min.X + pad, Y: top}, Size: Small}
		return s
	}
	wrapped := make([][]string, len(ordered))
	heights := make([]float64, len(ordered))
	for i, e := range ordered {
		text := e.Title
		if text == "" {
			text = e.SessionID
		}
		wrapped[i] = wrapAt(string(e.Kind)+" · "+text, inner, streamRows, Small, measure)
		heights[i] = float64(len(wrapped[i]))*lineH + grid
	}
	mark := lineH + grid
	used := 0.0
	s.Max = len(ordered)
	for i := len(ordered) - 1; i >= 0; i-- {
		room := bottom - top
		if i > 0 {
			room -= mark
		}
		if used+heights[i] > room {
			break
		}
		used += heights[i]
		s.Max = i
	}
	s.Max = min(s.Max, len(ordered)-1)
	s.Skip = min(max(skip, 0), s.Max)
	y := top
	x := area.Min.X + pad
	if s.Skip > 0 {
		s.Newer = Text{Text: fmt.Sprintf("↑ %d newer", s.Skip), At: city.Point{X: x, Y: y}, Size: Small}
		s.NewerRect = city.RectAt(area.Min.X+grid, y-grid/2, area.Width()-2*grid, mark)
		y += mark
	}
	for i := s.Skip; i < len(ordered); i++ {
		e, rowH := ordered[i], heights[i]
		if y+rowH > bottom {
			break
		}
		row := StreamRow{
			Event: e,
			Rect:  city.RectAt(area.Min.X+grid, y-grid/2, area.Width()-2*grid, rowH),
			Time:  Text{Text: e.At.Local().Format("15:04"), At: city.Point{X: x, Y: y}, Size: Small},
			Muted: !city.Flagged(e.Kind),
		}
		for j, line := range wrapped[i] {
			row.Lines = append(row.Lines, Text{Text: line, At: city.Point{X: x + timeW + grid, Y: y + float64(j)*lineH}, Size: Small})
		}
		s.Rows = append(s.Rows, row)
		y += rowH
	}
	if s.Max > 0 {
		s.Track = city.RectAt(area.Max.X-1.5*grid, top, grid/2, bottom-top)
		thumbH := max(s.Track.Height()*float64(len(s.Rows))/float64(len(ordered)), 2*grid)
		at := (s.Track.Height() - thumbH) * float64(s.Skip) / float64(s.Max)
		s.Thumb = city.RectAt(s.Track.Min.X, s.Track.Min.Y+at, s.Track.Width(), thumbH)
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
