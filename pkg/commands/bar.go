package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
	"time"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/format"
	"botropolis.city/botropolis/pkg/state"
)

type BarFormat string

const (
	BarWaybar BarFormat = "waybar"
	BarText   BarFormat = "text"
)

type BarLine struct {
	Text       string `json:"text"`
	Tooltip    string `json:"tooltip"`
	Class      string `json:"class"`
	Percentage int    `json:"percentage"`
}

type Bar struct {
	source SnapshotSource
	feed   func(ctx context.Context, offer func(state.Snapshot)) error
	layout func() *city.Layout
	now    func() time.Time
}

func NewBar(source SnapshotSource, feed func(ctx context.Context, offer func(state.Snapshot)) error) *Bar {
	return &Bar{source: source, feed: feed, layout: func() *city.Layout { return nil }, now: time.Now}
}

// WithTriage leaves out the sessions triage has set aside, reading the
// layout afresh for every line.
func (b *Bar) WithTriage(layout func() *city.Layout, now func() time.Time) *Bar {
	b.layout, b.now = layout, now
	return b
}

func (b *Bar) summarize(snapshot state.Snapshot) BarLine {
	return Summarize(snapshot, b.layout(), b.now())
}

func (b *Bar) Once(out io.Writer, format BarFormat, direct bool) error {
	snapshot, err := b.source.Snapshot(direct)
	if err != nil {
		return err
	}
	return writeBar(out, format, b.summarize(snapshot))
}

func (b *Bar) Watch(ctx context.Context, out io.Writer, format BarFormat) error {
	if b.feed == nil {
		return nil
	}
	return b.feed(ctx, func(snapshot state.Snapshot) {
		_ = writeBar(out, format, b.summarize(snapshot))
	})
}

func writeBar(out io.Writer, format BarFormat, line BarLine) error {
	if format == BarText {
		_, err := fmt.Fprintln(out, line.Text)
		return err
	}
	return json.NewEncoder(out).Encode(line)
}

func Summarize(snapshot state.Snapshot, layout *city.Layout, now time.Time) BarLine {
	counts := map[state.State]int{}
	var needs []state.Session
	var busiest float64
	setAside := 0
	for _, s := range snapshot.Sessions {
		if !state.Live(s.State) {
			continue
		}
		switch {
		case s.State == state.NeedsYou && layout.SetAside(s, now):
			setAside++
		case s.State == state.NeedsYou:
			counts[s.State]++
			needs = append(needs, s)
		default:
			counts[s.State]++
		}
		if s.ContextPercent > busiest {
			busiest = s.ContextPercent
		}
	}
	slices.SortFunc(needs, state.LongestWaiting)
	var parts []string
	for _, c := range state.Nonzero(counts, state.Live) {
		parts = append(parts, city.CountLabel(c))
	}
	if setAside > 0 {
		parts = append(parts, fmt.Sprintf("%d set aside", setAside))
	}
	line := BarLine{Text: strings.Join(parts, " · "), Percentage: int(math.Min(100, math.Max(0, busiest)) + 0.5)}
	switch {
	case counts[state.NeedsYou] > 0:
		line.Class = string(state.NeedsYou)
		line.Text = "● " + line.Text
	case counts[state.Working]+counts[state.Waiting]+counts[state.Unattended] > 0:
		line.Class = string(state.Working)
		line.Text = "○ " + line.Text
	case setAside > 0:
		line.Class = "idle"
	default:
		line.Class = "idle"
		line.Text = "no sessions"
	}
	if len(needs) > 0 {
		titles := make([]string, len(needs))
		for i, s := range needs {
			titles[i] = s.Title
		}
		line.Tooltip = "needs you:\n" + strings.Join(titles, "\n")
		// The bar has room for a name: say who is first in line.
		line.Text += " — " + format.Clip(titles[0], BarTitleWidth)
	}
	return line
}

// BarTitleWidth caps the needs-you title on the bar line.
const BarTitleWidth = 28
