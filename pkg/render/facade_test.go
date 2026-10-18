package render

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
)

// Item 86: planks are nailed across a shuttered building's own windows,
// covering each window's frame and sill, not only its glass.
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
			top, bottom, left, right := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
			for _, p := range planks {
				for _, corner := range p {
					top, bottom = math.Min(top, corner.Y), math.Max(bottom, corner.Y)
					left, right = math.Min(left, corner.X), math.Max(right, corner.X)
				}
			}

			t.Run("it should lay three planks across it", func(t *testing.T) {
				assert.Len(t, planks, 3)
			})

			for i, p := range planks {
				run := p[1].Sub(p[0])

				t.Run("it should run each plank near the face's slope", func(t *testing.T) {
					assert.InDelta(t, c.slope, run.Y/run.X, 0.15, "plank %d", i)
				})
			}

			t.Run("it should cover the frame on both sides of the glass", func(t *testing.T) {
				assert.Less(t, left, 100-0.25*20)
				assert.Greater(t, right, 120+0.25*20)
			})

			t.Run("it should reach above the window", func(t *testing.T) {
				assert.Less(t, top, 200-math.Abs(20*c.slope)/2)
			})

			t.Run("it should cover the sill below it", func(t *testing.T) {
				assert.Greater(t, bottom, 230+math.Abs(20*c.slope)/2+0.15*30)
			})
		})
	}

	// Item 109: the industrial kinds' ribbon windows run a whole face, and
	// boards reaching a share of that past the frame stood out past the
	// building's corner.
	t.Run("when a pane far wider than it is tall is boarded", func(t *testing.T) {
		pane := [4]city.Point{{X: 100, Y: 200}, {X: 300, Y: 300}, {X: 300, Y: 306}, {X: 100, Y: 206}}
		right := math.Inf(-1)
		for _, p := range planksFor(pane, 5) {
			for _, corner := range p {
				right = math.Max(right, corner.X)
			}
		}

		t.Run("it should reach past its frame by no more than the pane is tall", func(t *testing.T) {
			assert.LessOrEqual(t, right, 300+2*6.0)
		})
	})

	t.Run("when the same pane is boarded twice", func(t *testing.T) {
		pane := [4]city.Point{{X: 0, Y: 0}, {X: 20, Y: 10}, {X: 20, Y: 40}, {X: 0, Y: 30}}

		t.Run("it should nail the planks the same way", func(t *testing.T) {
			assert.Equal(t, planksFor(pane, 3), planksFor(pane, 3))
		})
	})
}
