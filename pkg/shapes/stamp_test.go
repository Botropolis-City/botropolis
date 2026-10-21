package shapes

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscStamp(t *testing.T) {
	t.Run("when a radius is asked for", func(t *testing.T) {
		radii := []float64{0.3, 0.5, 1, 1.2, 1.5, 2, 2.5, 3.7, 6, 9.9, 14, 22, 40, 77, discRadii[len(discRadii)-1]}
		for _, r := range radii {
			level, scale, ok := discStamp(r)
			require.True(t, ok, "radius %v", r)

			t.Run("it should pick a stamp no smaller than the radius", func(t *testing.T) {
				require.GreaterOrEqual(t, discRadii[level]+1e-9, r)
			})

			t.Run("it should shrink no stamp by more than √2", func(t *testing.T) {
				require.GreaterOrEqual(t, scale, 1/math.Sqrt2-1e-9, "radius %v", r)
			})

			t.Run("it should scale the stamp to the radius", func(t *testing.T) {
				require.InDelta(t, r, discRadii[level]*scale, 1e-9)
			})
		}
	})

	t.Run("when a radius is larger than the largest stamp", func(t *testing.T) {
		biggest := discRadii[len(discRadii)-1]
		level, scale, _ := discStamp(biggest * 3)

		t.Run("it should scale the largest up", func(t *testing.T) {
			assert.Equal(t, []any{len(discRadii) - 1, 3.0}, []any{level, scale})
		})
	})

	for _, r := range []float64{0, -2, math.NaN()} {
		t.Run(fmt.Sprintf("when the radius is %v", r), func(t *testing.T) {
			_, _, ok := discStamp(r)

			t.Run("it should draw nothing", func(t *testing.T) {
				assert.False(t, ok, "radius %v", r)
			})
		})
	}
}
