package ui

import (
	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/events"
)

// TimelineRow is one event on the timeline.
type TimelineRow struct {
	Rect      city.Rect
	Time      Text
	Dot       city.Rect
	Tone      Tone
	Title     Text
	Detail    Text
	SessionID string
	Selected  bool
}

// Timeline is the event log (or the away list) laid out on a panel.
type Timeline struct {
	Rect     city.Rect
	Title    Text
	Subtitle Text
	Rows     []TimelineRow
}

const timelineGrids = 80

// EventTone is the tone an event kind is drawn in.
func EventTone(kind events.Kind) Tone {
	switch kind {
	case events.NeedsYou:
		return ToneNeedsYou
	case events.Error:
		return ToneError
	case events.Merged:
		return ToneMerged
	case events.PR, events.Started:
		return ToneWorking
	case events.Ended:
		return ToneResting
	}
	return ToneNone
}

// eventWords says what an event was, for its row.
func eventWords(e events.Event) string {
	switch e.Kind {
	case events.NeedsYou:
		if e.Detail != "" {
			return "needs you: " + e.Detail
		}
		return "needs you"
	case events.Error:
		return e.Detail
	case events.PR:
		return "opened " + e.Detail
	case events.Merged:
		return "merged " + e.Detail
	case events.Compaction:
		return e.Detail
	case events.Started:
		return "started " + e.Detail
	case events.Ended:
		return "ended (" + e.Detail + ")"
	}
	return string(e.Kind)
}

// LayoutTimeline centres a titled list of events, newest first, with
// the cursor's row flagged; rows past the bottom are dropped.
func LayoutTimeline(th Theme, width, height float64, title string, list []events.Event, cursor int, measure Measure) Timeline {
	grid := th.Grid()
	pad := 3 * grid
	_, titleH := measure(title, Title)
	titleH = subtitled(titleH, measure)
	_, bodyH := measure("", Body)
	_, smallH := measure("", Small)
	rowH := bodyH + grid
	w := timelineGrids * grid
	if w > width-4*grid {
		w = width - 4*grid
	}
	maxRows := int((height - 4*grid - pad - titleH - grid - pad) / rowH)
	if maxRows < 1 {
		maxRows = 1
	}
	n := min(len(list), maxRows)
	if n == 0 {
		n = 1
	}
	h := pad + titleH + grid + rowH*float64(n) + pad
	tl := Timeline{Rect: city.RectAt((width-w)/2, (height-h)/2, w, h)}
	x0 := tl.Rect.Min.X + pad
	tl.Title = Text{Text: title, At: city.Point{X: x0, Y: tl.Rect.Min.Y + pad}, Size: Title}
	tl.Subtitle = subtitleUnder(tl.Title, PanelTimeline, measure)
	y := tl.Rect.Min.Y + pad + titleH + grid
	if len(list) == 0 {
		tl.Rows = append(tl.Rows, TimelineRow{
			Rect:  city.RectAt(tl.Rect.Min.X, y, w, rowH),
			Title: Text{Text: "nothing yet", At: city.Point{X: x0, Y: y + grid/2}, Size: Body},
		})
		return tl
	}
	timeW, _ := measure("00:00", Small)
	titleW := 28 * grid
	for i, e := range list {
		if i == maxRows {
			break
		}
		detail := clipTo(eventWords(e), w-2*pad-timeW-grid-grid-grid-titleW-grid, Small, measure)
		tl.Rows = append(tl.Rows, TimelineRow{
			Rect:      city.RectAt(tl.Rect.Min.X, y, w, rowH),
			Time:      Text{Text: e.At.Format("15:04"), At: city.Point{X: x0, Y: y + grid/2 + (bodyH-smallH)/2}, Size: Small},
			Dot:       city.RectAt(x0+timeW+grid, y+(rowH-grid)/2, grid, grid),
			Tone:      EventTone(e.Kind),
			Title:     Text{Text: clipTo(e.Title, titleW, Body, measure), At: city.Point{X: x0 + timeW + 3*grid, Y: y + grid/2}, Size: Body},
			Detail:    Text{Text: detail, At: city.Point{X: x0 + timeW + 3*grid + titleW + grid, Y: y + grid/2 + (bodyH-smallH)/2}, Size: Small},
			SessionID: e.SessionID,
			Selected:  i == cursor,
		})
		y += rowH
	}
	return tl
}

// Hit is the row under a point, if any.
func (t Timeline) Hit(at city.Point) (TimelineRow, bool) {
	for _, r := range t.Rows {
		if r.Rect.Contains(at) {
			return r, true
		}
	}
	return TimelineRow{}, false
}
