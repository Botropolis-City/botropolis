package ui_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/stats"
	"botropolis.city/botropolis/pkg/ui"
)

func chipTexts(chips []ui.Chip) []string {
	out := make([]string, 0, len(chips))
	for _, c := range chips {
		out = append(out, c.Text)
	}
	return out
}

func findChip(chips []ui.Chip, text string) (ui.Chip, bool) {
	for _, c := range chips {
		if c.Text == text {
			return c, true
		}
	}
	return ui.Chip{}, false
}

func TestStripChips(t *testing.T) {
	t.Run("when the summary is quiet", func(t *testing.T) {
		chips := ui.StripChips(city.Summary{Working: 2, Resting: 5, FreshPerH: 1500, CachedPerH: 20000, OutputPerH: 700, Cost: stats.Cost{Low: 1.5, High: 1.5}, Priced: true, HitRatio: 0.9, MCPCalls: 3})

		t.Run("it should give output its own chip", func(t *testing.T) {
			_, ok := findChip(chips, "700/h out")
			assert.True(t, ok)
		})

		t.Run("it should call the mcp chip a count of calls (item 181)", func(t *testing.T) {
			_, ok := findChip(chips, "3 mcp calls")
			assert.True(t, ok, chipTexts(chips))
		})

		t.Run("it should lead with the working chip", func(t *testing.T) {
			assert.Equal(t, "2 working", chips[0].Text)
		})

		t.Run("it should tone the working chip working", func(t *testing.T) {
			assert.Equal(t, ui.ToneWorking, chips[0].Tone)
		})

		t.Run("it should make the working chip jump to working sessions", func(t *testing.T) {
			assert.Equal(t, state.Working, chips[0].State)
		})

		for _, absent := range []string{"need you", "unattended", "subagent", "PR", "error"} {
			t.Run("it should omit the "+absent+" chip", func(t *testing.T) {
				assert.NotContains(t, fmt.Sprint(chipTexts(chips)), absent)
			})
		}

		for _, plain := range []string{"2k/h fresh", "20k/h cached", "$1.50 24h", "hit 90%", "3 mcp calls"} {
			t.Run("it should leave "+plain+" without a tone", func(t *testing.T) {
				chip, ok := findChip(chips, plain)
				require.True(t, ok, chipTexts(chips))
				assert.Equal(t, ui.ToneNone, chip.Tone)
			})
		}

		t.Run("it should leave the token chips with no jump", func(t *testing.T) {
			chip, ok := findChip(chips, "2k/h fresh")
			require.True(t, ok)
			assert.Empty(t, chip.State)
		})
	})

	t.Run("when sessions are set aside", func(t *testing.T) {
		chips := ui.StripChips(city.Summary{NeedsYou: 1, Working: 1, SetAside: 1})

		t.Run("it should show a plain set-aside chip after the state chips", func(t *testing.T) {
			assert.Equal(t, ui.Chip{Text: "1 set aside"}, chips[2])
		})
	})

	t.Run("when a daily budget is set", func(t *testing.T) {
		cases := []struct {
			cost float64
			text string
			tone ui.Tone
		}{
			{100, "$100.00 of $250 24h", ui.ToneNone},
			{210, "$210.00 of $250 24h", ui.ToneNeedsYou},
			{260, "$260.00 of $250 24h", ui.ToneError},
		}
		for _, tc := range cases {
			chips := ui.StripChips(city.Summary{Working: 1, Cost: stats.Cost{Low: tc.cost, High: tc.cost}, Priced: true, BudgetUSD: 250})
			chip, ok := findChip(chips, tc.text)
			require.True(t, ok, chipTexts(chips))

			t.Run(fmt.Sprintf("it should tone %q %s", tc.text, tc.tone), func(t *testing.T) {
				assert.Equal(t, tc.tone, chip.Tone)
			})
		}
	})

	t.Run("when some writes have an unknown lifetime, it should give the cost as a range", func(t *testing.T) {
		chips := ui.StripChips(city.Summary{Working: 1, Cost: stats.Cost{Low: 5, High: 8}, Priced: true})
		_, ok := findChip(chips, "$5.00–$8.00 24h")
		assert.True(t, ok, chipTexts(chips))
	})

	reasons := []struct {
		name, reason, chip string
	}{
		{"when statistics are paused, briefly, leaving the reason to the plant", "the store is being upgraded", "statistics paused"},
		{"when statistics are off", "statistics are off", "statistics off"},
	}
	for _, tc := range reasons {
		t.Run(tc.name, func(t *testing.T) {
			chips := ui.StripChips(city.Summary{Working: 1, BudgetUSD: 250, Reason: tc.reason})

			t.Run("it should give no cost chip", func(t *testing.T) {
				assert.NotContains(t, strings.Join(chipTexts(chips), " "), "$")
			})

			t.Run("it should say why there is none", func(t *testing.T) {
				_, ok := findChip(chips, tc.chip)
				assert.True(t, ok, chipTexts(chips))
			})
		})
	}

	t.Run("when statistics can be read, it should give no statistics chip", func(t *testing.T) {
		chips := ui.StripChips(city.Summary{Working: 1, Priced: true})
		assert.NotContains(t, strings.Join(chipTexts(chips), " "), "statistics")
	})

	t.Run("when every state has sessions", func(t *testing.T) {
		chips := ui.StripChips(city.Summary{Working: 2, Resting: 3, NeedsYou: 1, Unattended: 4})

		t.Run("it should lead with needs-you and run down the urgency order", func(t *testing.T) {
			assert.Equal(t, []string{"1 needs you", "2 working", "4 unattended", "3 resting"}, chipTexts(chips)[:4])
		})
	})

	t.Run("when nothing is working", func(t *testing.T) {
		chips := ui.StripChips(city.Summary{NeedsYou: 1, Resting: 3})

		t.Run("it should drop the working chip and keep the slot order", func(t *testing.T) {
			assert.Equal(t, []string{"1 needs you", "3 resting"}, chipTexts(chips)[:2])
		})
	})

	t.Run("when the summary has something to point at", func(t *testing.T) {
		cases := []struct {
			summary city.Summary
			text    string
			tone    ui.Tone
			state   state.State
		}{
			{city.Summary{NeedsYou: 1}, "1 needs you", ui.ToneNeedsYou, state.NeedsYou},
			{city.Summary{NeedsYou: 2}, "2 need you", ui.ToneNeedsYou, state.NeedsYou},
			{city.Summary{Unattended: 2}, "2 unattended", ui.ToneUnattended, state.Unattended},
			{city.Summary{Resting: 3}, "3 resting", ui.ToneResting, state.Resting},
			{city.Summary{Subagents: 1}, "1 subagent", ui.ToneNone, ""},
			{city.Summary{Subagents: 4}, "4 subagents", ui.ToneNone, ""},
			{city.Summary{PRs: 1}, "1 PR", ui.ToneMerged, ""},
			{city.Summary{PRs: 5}, "5 PRs", ui.ToneMerged, ""},
			{city.Summary{Errors: 1}, "1 error", ui.ToneError, ""},
			{city.Summary{Errors: 6}, "6 errors", ui.ToneError, ""},
		}
		for _, tc := range cases {
			chips := ui.StripChips(tc.summary)
			chip, ok := findChip(chips, tc.text)
			require.True(t, ok, chipTexts(chips))

			t.Run(fmt.Sprintf("it should tone %q %s", tc.text, tc.tone), func(t *testing.T) {
				assert.Equal(t, tc.tone, chip.Tone)
			})

			t.Run(fmt.Sprintf("it should give %q the jump %q", tc.text, tc.state), func(t *testing.T) {
				assert.Equal(t, tc.state, chip.State)
			})
		}
	})
}

func measure7(s string, _ ui.Size) (float64, float64) {
	return float64(len([]rune(s))) * 7, 16
}

func TestLayoutStrip(t *testing.T) {
	th := ui.NewTheme(1)
	chips := []ui.Chip{
		{Text: "2 working", Tone: ui.ToneWorking, State: state.Working},
		{Text: "2k/h fresh"},
		{Text: "1 error", Tone: ui.ToneError},
	}

	t.Run("when the chips fit", func(t *testing.T) {
		strip := ui.LayoutStrip(th, chips, 800, measure7)

		t.Run("it should keep every chip", func(t *testing.T) {
			assert.Len(t, strip.Chips, 3)
		})

		t.Run("it should make the strip four grid units tall", func(t *testing.T) {
			assert.Equal(t, 32.0, strip.Height)
		})

		t.Run("it should start the first dot two grid units in", func(t *testing.T) {
			assert.Equal(t, 16.0, strip.Chips[0].Dot.Min.X)
		})

		t.Run("it should make the dot one grid unit wide", func(t *testing.T) {
			assert.Equal(t, 8.0, strip.Chips[0].Dot.Width())
		})

		t.Run("it should centre the dot on the strip", func(t *testing.T) {
			assert.Equal(t, 16.0, strip.Chips[0].Dot.Center().Y)
		})

		t.Run("it should put the text one grid unit after the dot", func(t *testing.T) {
			assert.Equal(t, 32.0, strip.Chips[0].TextAt.X)
		})

		t.Run("it should centre the text on the strip", func(t *testing.T) {
			assert.Equal(t, 8.0, strip.Chips[0].TextAt.Y)
		})

		t.Run("it should give a plain chip no dot", func(t *testing.T) {
			assert.Zero(t, strip.Chips[1].Dot.Area())
		})

		t.Run("it should start a plain chip's text three grid units after the last text", func(t *testing.T) {
			assert.Equal(t, 32.0+9*7+24, strip.Chips[1].TextAt.X)
		})

		t.Run("it should span a chip's hit box over its dot and text", func(t *testing.T) {
			assert.Equal(t, city.RectAt(12, 0, 8+8+9*7+8, 32), strip.Chips[0].Rect)
		})

		t.Run("it should measure at body size", func(t *testing.T) {
			assert.Equal(t, ui.Body, strip.Size)
		})
	})

	t.Run("when the chips overflow", func(t *testing.T) {
		strip := ui.LayoutStrip(th, chips, 220, measure7)

		t.Run("it should drop the chips past the edge", func(t *testing.T) {
			assert.Len(t, strip.Chips, 2)
		})
	})

	t.Run("when hit-testing", func(t *testing.T) {
		strip := ui.LayoutStrip(th, chips, 800, measure7)

		t.Run("it should find the state chip under the pointer", func(t *testing.T) {
			hit, ok := strip.Hit(strip.Chips[0].Rect.Center())
			require.True(t, ok)
			assert.Equal(t, state.Working, hit.State)
		})

		t.Run("it should ignore a chip with no jump under the pointer", func(t *testing.T) {
			_, ok := strip.Hit(strip.Chips[1].Rect.Center())
			assert.False(t, ok)
		})

		t.Run("it should find nothing below the strip", func(t *testing.T) {
			_, ok := strip.Hit(city.Point{X: 20, Y: 40})
			assert.False(t, ok)
		})
	})
}

// This test is load-bearing, and not in the way it looks.
//
// It reads like a tidiness check — fewer dots while a view is up — and
// it is not. The view palettes cannot be kept clear of the state tones
// by choosing hues: of the 56 ways to take three of the reference
// palette's eight dark slots, the 15 that pass a map's all-pairs gates
// all come within delta E 2.2 of some state tone under one of the
// deficiencies, and the sequential ramp passes within 0.3 of the waiting
// teal. The fourth condition the ramps were allowed under — that they
// stay distinct from the state tones — is therefore kept by the mode
// rather than by the palette: while a view is up the tones are not on
// screen, so the two can never be confused because they are never seen
// together.
//
// Which means that relaxing this assertion does not cost a little
// polish. It makes the palette illegal again, and no choice of hue
// repairs it. See rules/why-the-category-colours-are-legal.md for
// the search and the argument before changing anything here.
func TestDrained(t *testing.T) {
	t.Run("when a view is up and the strip has to give the tones back", func(t *testing.T) {
		summary := city.Summary{NeedsYou: 3, Working: 1, Resting: 2, MCPCalls: 9, PRs: 3, Errors: 1}
		before := ui.StripChips(summary)
		after := ui.Drained(before)

		t.Run("it should leave no state tone anywhere on the strip", func(t *testing.T) {
			for _, c := range after {
				require.Equal(t, ui.ToneNone, c.Tone, c.Text)
			}
		})

		t.Run("it should keep every chip, because the counts still answer the question", func(t *testing.T) {
			assert.Equal(t, chipTexts(before), chipTexts(after))
		})

		t.Run("it should keep the jumps, because navigation is not colour", func(t *testing.T) {
			chip, ok := findChip(after, "3 need you")
			require.True(t, ok)
			assert.Equal(t, state.NeedsYou, chip.State)
		})

		t.Run("it should not disturb the row it was given", func(t *testing.T) {
			assert.Equal(t, ui.ToneNeedsYou, before[0].Tone)
		})
	})
}
