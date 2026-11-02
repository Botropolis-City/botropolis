package measure

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Item 330: a stress reading is traced by the work its ticks did and the
// cores they ran on, since its thread CPU moves with both.
func TestTally(t *testing.T) {
	counted := func(instructions uint64, core int) Cost {
		return Cost{Instructions: instructions, Counted: true, EndedOn: core}
	}
	for _, c := range []struct {
		scenario string
		costs    []Cost
		said     string
	}{
		{
			scenario: "when the ticks were counted on several cores",
			costs:    []Cost{counted(131e6, 11), counted(130e6, 0), counted(129e6, 0)},
			said:     "130.0 M instructions a tick (median of 3), ended on cores 0×2 11×1",
		},
		{
			scenario: "when the ticks were not counted",
			costs:    []Cost{{EndedOn: 3}},
			said:     "instructions not counted (1 tick), ended on cores 3×1",
		},
		{
			scenario: "when a tick's core could not be read",
			costs:    []Cost{counted(1e6, -1)},
			said:     "1.0 M instructions a tick (median of 1), ended on cores unknown×1",
		},
		{
			scenario: "when there were no ticks",
			said:     "no ticks",
		},
	} {
		t.Run(c.scenario, func(t *testing.T) {
			t.Run("it should say so", func(t *testing.T) {
				assert.Equal(t, c.said, Tally(c.costs))
			})
		})
	}
}
