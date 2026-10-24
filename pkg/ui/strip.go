package ui

import (
	"fmt"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/format"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/stats"
)

// Measure is how wide and tall a string is at a type size on the display;
// the renderer supplies it so the layout stays pure.
type Measure func(s string, size Size) (width, height float64)

// Chip is one tally on the resource strip. A tone gives it a coloured dot;
// a state makes clicking it jump to the next session in that state.
type Chip struct {
	Text  string
	Tone  Tone
	State state.State
}

// StripChips is the summary as a row of chips: sessions by state in
// state.Order with zero counts left out, then tokens and cost, then the rest.
func StripChips(s city.Summary) []Chip {
	var chips []Chip
	for _, c := range state.Nonzero(s.Counts()) {
		chips = append(chips, Chip{city.CountLabel(c), StateTone(c.State), c.State})
	}
	if s.SetAside > 0 {
		chips = append(chips, Chip{Text: fmt.Sprintf("%d set aside", s.SetAside)})
	}
	chips = append(chips,
		Chip{Text: format.Tokens(s.FreshPerH) + "/h fresh"},
		Chip{Text: format.Tokens(s.CachedPerH) + "/h cached"},
		Chip{Text: format.Tokens(s.OutputPerH) + "/h out"},
	)
	if s.Priced {
		chips = append(chips, costChip(s))
	} else {
		chips = append(chips, statsChip(s))
	}
	chips = append(chips,
		Chip{Text: "hit " + format.Percent(100*s.HitRatio)},
	)
	if s.Subagents > 0 {
		chips = append(chips, Chip{Text: format.Count(s.Subagents, "subagent", "subagents")})
	}
	chips = append(chips, Chip{Text: fmt.Sprintf("%d mcp calls", s.MCPCalls)})
	if s.PRs > 0 {
		chips = append(chips, Chip{Text: format.Count(s.PRs, "PR", "PRs"), Tone: ToneMerged})
	}
	if s.Errors > 0 {
		chips = append(chips, Chip{Text: format.Count(s.Errors, "error", "errors"), Tone: ToneError})
	}
	return chips
}

// statsChip says why there is no cost chip, briefly, so its absence
// reads as a choice and not a fault: statistics are off, or paused, with
// the reason in the plant's panel.
func statsChip(s city.Summary) Chip {
	if s.Reason == stats.ReasonOff {
		return Chip{Text: "statistics off"}
	}
	return Chip{Text: "statistics paused"}
}

// costChip is the 24 h cost at list prices, a range while some writes'
// lifetime is unknown, measured against the daily budget when there is
// one: plain under 80 % of it, accent-toned up to it, error beyond.
func costChip(s city.Summary) Chip {
	cost := stats.Dollars(s.Cost)
	if s.BudgetUSD <= 0 {
		return Chip{Text: cost + " 24h"}
	}
	chip := Chip{Text: fmt.Sprintf("%s of $%.0f 24h", cost, s.BudgetUSD)}
	switch share := s.BudgetShare(); {
	case share >= 1:
		chip.Tone = ToneError
	case share >= 0.8:
		chip.Tone = ToneNeedsYou
	}
	return chip
}

// PlacedChip is a chip with everything the renderer needs to draw it and
// the pointer needs to find it.
type PlacedChip struct {
	Chip
	Rect   city.Rect
	Dot    city.Rect
	TextAt city.Point
}

// Strip is the resource strip laid out for one window width.
type Strip struct {
	Height float64
	Size   Size
	Chips  []PlacedChip
}

// LayoutStrip places chips left to right on the grid, dropping the ones
// that would run past the right edge.
func LayoutStrip(th Theme, chips []Chip, width float64, measure Measure) Strip {
	grid := th.Grid()
	strip := Strip{Height: 4 * grid, Size: Body}
	x := 2 * grid
	for _, chip := range chips {
		start := x
		placed := PlacedChip{Chip: chip}
		if chip.Tone != ToneNone {
			placed.Dot = city.RectAt(x, (strip.Height-grid)/2, grid, grid)
			x += 2 * grid
		}
		w, h := measure(chip.Text, strip.Size)
		placed.TextAt = city.Point{X: x, Y: (strip.Height - h) / 2}
		x += w
		if x > width-2*grid {
			break
		}
		placed.Rect = city.RectAt(start-grid/2, 0, x-start+grid, strip.Height)
		strip.Chips = append(strip.Chips, placed)
		x += 3 * grid
	}
	return strip
}

// Hit is the chip with a jump under the pointer, if any.
func (s Strip) Hit(at city.Point) (Chip, bool) {
	for _, c := range s.Chips {
		if c.State != "" && c.Rect.Contains(at) {
			return c.Chip, true
		}
	}
	return Chip{}, false
}

// Drained is the strip with its state tones taken off, which is what a
// view does to it.
//
// This is not decoration. The state palette spends amber, blue, teal,
// violet, slate, red and green, and a view's own palette cannot stay
// clear of all seven — the closest any validated three gets is delta E
// 2.2 under deuteranopia, and the sequential ramp passes within 0.3 of
// the waiting teal. So the promise that one colour means one thing is
// kept by the mode rather than by the palette: while a view is up the
// tones are not on screen, and the only colour is the view's.
//
// The counts and their jumps stay. Who wants you is still the question a
// glance asks, and clicking a count to reach the next session is
// navigation, not colour.
func Drained(chips []Chip) []Chip {
	out := make([]Chip, len(chips))
	for i, c := range chips {
		c.Tone = ToneNone
		out[i] = c
	}
	return out
}
