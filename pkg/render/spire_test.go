package render

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// Item 109, slice 5: past the top line, a thin spire rises from the
// roof, a segment per doubling, with a beacon at its tip.
func TestSpire(t *testing.T) {
	base := city.Point{X: 100, Y: 200}
	storey := 30.0

	for _, n := range []int{1, 3, 6} {
		t.Run(fmt.Sprintf("when a building has a spire of %d segments", n), func(t *testing.T) {
			s := spireOf(base, storey, n)

			t.Run("it should rise one segment height per segment above the roof", func(t *testing.T) {
				assert.InDelta(t, float64(n)*spireSegment(storey), base.Y-s.tip.Y, 1e-9)
			})

			t.Run("it should draw one stroke per segment", func(t *testing.T) {
				assert.Len(t, s.segments, n)
			})

			t.Run("it should carry a beacon at its tip", func(t *testing.T) {
				assert.Equal(t, [2]any{true, s.tip}, [2]any{s.beacon, s.segments[n-1].b})
			})
		})
	}

	t.Run("when a building has no spire", func(t *testing.T) {
		s := spireOf(base, storey, 0)

		t.Run("it should draw none", func(t *testing.T) {
			assert.Equal(t, [2]any{0, false}, [2]any{len(s.segments), s.beacon})
		})
	})
}
