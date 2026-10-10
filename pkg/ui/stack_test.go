package ui

import (
	"math"
	"testing"

	"botropolis.city/botropolis/pkg/city"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Bugs 46, 46a and 46b. A cylinder cut by a horizontal plane meets it
// in a circle, which in this projection is an ellipse twice as wide as
// it is tall; what a short stack on a flat roof carries at that join is
// a low curb; and a ring wider than the stack wraps well past the
// horizontal diameter.

func stackRect(width float64) (city.Rect, float64) {
	r := city.RectAt(100, 60, width, 90)
	return r, r.Max.Y
}

func TestHiddenArc(t *testing.T) {
	t.Run("when the ring is half again as wide as the stack", func(t *testing.T) {
		t.Run("it should hide 80 degrees of ring, not 180", func(t *testing.T) {
			assert.InDelta(t, 80.4, degrees(HiddenArc(1, StackCurbRatio)), 0.1)
		})

		t.Run("it should leave the ring's ends tucking behind the stack, not stopping in mid air", func(t *testing.T) {
			assert.InDelta(t, 139.8, 180-degrees(math.Asin(1/StackCurbRatio)), 0.1)
		})
	})

	t.Run("when the ring is only a third wider, as bug 46b had it", func(t *testing.T) {
		t.Run("it should still hide less than half, which was never the problem", func(t *testing.T) {
			assert.InDelta(t, 98.5, degrees(HiddenArc(1, 1.32)), 0.1)
		})
	})

	t.Run("when the ring is no wider than the stack", func(t *testing.T) {
		t.Run("it should hide half of it, which is the only case where half was ever right", func(t *testing.T) {
			assert.InDelta(t, 180.0, degrees(HiddenArc(1, 1)), 0.001)
		})
	})
}

func TestLayoutStackFittings(t *testing.T) {
	stack, roof := stackRect(87)
	f, ok := LayoutStackFittings(stack, roof)
	require.True(t, ok)
	pipe := stack.Width() / 2 * StackPipeShare

	t.Run("when the curb rings the opening", func(t *testing.T) {
		t.Run("it should be sized against the pipe, not the sprite's flared width", func(t *testing.T) {
			assert.InDelta(t, pipe*StackCurbRatio, f.OuterFoot.Width()/2, 0.0001)
		})

		t.Run("it should be at least root two wider than the pipe, or the corners cannot be covered", func(t *testing.T) {
			assert.GreaterOrEqual(t, StackCurbRatio, math.Sqrt2)
		})

		t.Run("it should have a feasible height band at all, which below root two it does not", func(t *testing.T) {
			assert.Greater(t, CurbCeiling(pipe, pipe*StackCurbRatio), CurbFloor(pipe))
		})

		t.Run("it should meet the stack wall with no gap", func(t *testing.T) {
			assert.InDelta(t, pipe, f.InnerTop.Width()/2, 0.0001)
		})

		t.Run("it should hold one width top and bottom, being a ring and not a cone", func(t *testing.T) {
			assert.InDelta(t, f.OuterFoot.Width(), f.OuterTop.Width(), 0.0001)
		})
	})

	t.Run("when the curb has to hide the sprite's straight cut", func(t *testing.T) {
		height := f.OuterFoot.Center().Y - f.OuterTop.Center().Y
		outer := f.OuterFoot.Width() / 2

		t.Run("it should stand tall enough to hide the cut at the centre", func(t *testing.T) {
			assert.GreaterOrEqual(t, height, CurbFloor(pipe))
		})

		t.Run("it should stay low enough to cover the sprite's square corners", func(t *testing.T) {
			assert.LessOrEqual(t, height, CurbCeiling(pipe, outer))
		})

		t.Run("it should put the new bottom line below the cut at its lowest, which is the bulge", func(t *testing.T) {
			assert.Greater(t, f.InnerTop.Max.Y, f.InnerTop.Center().Y)
		})
	})

	t.Run("when every ellipse is laid on the ground plane", func(t *testing.T) {
		for _, e := range []city.Rect{f.OuterFoot, f.OuterTop, f.InnerTop} {
			t.Run("it should be twice as wide as it is tall", func(t *testing.T) {
				assert.InDelta(t, 2.0, e.Width()/e.Height(), 0.0001)
			})
		}
	})

	t.Run("when there is no stack to ring", func(t *testing.T) {
		t.Run("it should decline", func(t *testing.T) {
			_, ok := LayoutStackFittings(city.Rect{}, 0)
			assert.False(t, ok)
		})
	})
}

func degrees(rad float64) float64 { return rad * 180 / math.Pi }
