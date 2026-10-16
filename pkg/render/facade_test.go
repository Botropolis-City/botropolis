package render

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
)

// Item 86: planks are nailed across a shuttered building's own windows.
func TestPlanks(t *testing.T) {
	for _, c := range []struct {
		face  string
		slope float64
	}{
		{"left", 0.5},
		{"right", -0.5},
	} {
		t.Run("when a pane on the "+c.face+" face is boarded", func(t *testing.T) {
			pane := [4]city.Point{
				{X: 100, Y: 200}, {X: 120, Y: 200 + 20*c.slope},
				{X: 120, Y: 230 + 20*c.slope}, {X: 100, Y: 230},
			}
			planks := planksFor(pane, 7)
			require.NotEmpty(t, planks)

			t.Run("it should lay two planks across it", func(t *testing.T) {
				assert.Len(t, planks, 2)
			})

			for i, p := range planks {
				run := p[1].Sub(p[0])

				t.Run("it should run each plank near the face's slope", func(t *testing.T) {
					assert.InDelta(t, c.slope, run.Y/run.X, 0.15, "plank %d", i)
				})

				t.Run("it should run each plank past both sides of the pane", func(t *testing.T) {
					assert.Less(t, math.Min(p[0].X, p[3].X), 100.0, "plank %d", i)
					assert.Greater(t, math.Max(p[1].X, p[2].X), 120.0, "plank %d", i)
				})

				t.Run("it should keep each plank within the pane's height", func(t *testing.T) {
					for _, corner := range p {
						assert.GreaterOrEqual(t, corner.Y, 200.0-20*math.Abs(c.slope)-1, "plank %d", i)
						assert.LessOrEqual(t, corner.Y, 230.0+20*math.Abs(c.slope)+1, "plank %d", i)
					}
				})
			}
		})
	}

	t.Run("when the same pane is boarded twice", func(t *testing.T) {
		pane := [4]city.Point{{X: 0, Y: 0}, {X: 20, Y: 10}, {X: 20, Y: 40}, {X: 0, Y: 30}}

		t.Run("it should nail the planks the same way", func(t *testing.T) {
			assert.Equal(t, planksFor(pane, 3), planksFor(pane, 3))
		})
	})
}
