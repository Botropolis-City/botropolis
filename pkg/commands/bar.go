package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

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
	feed   func(ctx context.Context, offer func(state.Snapshot))
}

func NewBar(source SnapshotSource, feed func(ctx context.Context, offer func(state.Snapshot))) *Bar {
	return &Bar{source: source, feed: feed}
}

func (b *Bar) Once(out io.Writer, format BarFormat, direct bool) error {
	snapshot, err := b.source.Snapshot(direct)
	if err != nil {
		return err
	}
	return writeBar(out, format, Summarize(snapshot))
}

func (b *Bar) Watch(ctx context.Context, out io.Writer, format BarFormat) {
	if b.feed == nil {
		return
	}
	b.feed(ctx, func(snapshot state.Snapshot) {
		_ = writeBar(out, format, Summarize(snapshot))
	})
}

func writeBar(out io.Writer, format BarFormat, line BarLine) error {
	if format == BarText {
		_, err := fmt.Fprintln(out, line.Text)
		return err
	}
	return json.NewEncoder(out).Encode(line)
}

func Summarize(snapshot state.Snapshot) BarLine {
	counts := map[state.State]int{}
	var needs []string
	var busiest float64
	for _, s := range snapshot.Sessions {
		if !state.Live(s.State) {
			continue
		}
		counts[s.State]++
		if s.State == state.NeedsYou {
			needs = append(needs, s.Title)
		}
		if s.ContextPercent > busiest {
			busiest = s.ContextPercent
		}
	}
	sort.Strings(needs)
	var parts []string
	for _, c := range state.Nonzero(counts, state.Live) {
		parts = append(parts, fmt.Sprintf("%d %s", c.N, c.State))
	}
	line := BarLine{Text: strings.Join(parts, " · "), Percentage: int(math.Min(100, math.Max(0, busiest)) + 0.5)}
	switch {
	case counts[state.NeedsYou] > 0:
		line.Class = string(state.NeedsYou)
		line.Text = "● " + line.Text
	case counts[state.Working]+counts[state.Waiting]+counts[state.Unattended] > 0:
		line.Class = string(state.Working)
		line.Text = "○ " + line.Text
	default:
		line.Class = "idle"
		line.Text = "no sessions"
	}
	if len(needs) > 0 {
		line.Tooltip = "needs you:\n" + strings.Join(needs, "\n")
		// The bar has room for a name: say who is first in line.
		line.Text += " — " + format.Clip(needs[0], BarTitleWidth)
	}
	return line
}

// BarTitleWidth caps the needs-you title on the bar line.
const BarTitleWidth = 28
