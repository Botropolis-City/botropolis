package render

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 112: each lot on the City Plan is tinted faintly by whose move it
// is, and the water beyond the coast is a wash no state tone shares.
func TestCityPlanCharacter(t *testing.T) {
	th := ui.NewTheme(1)
	rgb := func(c color.NRGBA) [3]uint8 { return [3]uint8{c.R, c.G, c.B} }

	for _, m := range []state.Move{state.MoveYours, state.MoveTheirs, state.MoveAgents, state.MoveNobody} {
		t.Run("a lot's tint, when it is "+moveName(m), func(t *testing.T) {
			tint := planLotTint(th, m)

			t.Run("it should be that move's tone", func(t *testing.T) {
				assert.Equal(t, rgb(th.Color(ui.MoveTone(m))), rgb(tint))
			})

			t.Run("it should be faint", func(t *testing.T) {
				assert.True(t, tint.A > 0 && tint.A <= 0x30, "alpha %#x", tint.A)
			})
		})
	}

	t.Run("the water", func(t *testing.T) {
		t.Run("it should be no state tone", func(t *testing.T) {
			var near []string
			for tone := ui.ToneNeedsYou; tone <= ui.ToneMerged; tone++ {
				c := th.Color(tone)
				d := absDiff(c.R, colorPlanWater.R) + absDiff(c.G, colorPlanWater.G) + absDiff(c.B, colorPlanWater.B)
				if d < 48 {
					near = append(near, tone.String())
				}
			}
			assert.Empty(t, near)
		})
	})
}

func moveName(m state.Move) string { return string(m) }

func absDiff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}
