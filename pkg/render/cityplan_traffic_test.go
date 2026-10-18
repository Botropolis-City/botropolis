package render

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
)

// Item 112: traffic on the City Plan is dashed ink.
func TestPlanDashes(t *testing.T) {
	const on, off = 6.0, 6.0
	length := func(a, b city.Point) float64 { return math.Hypot(b.X-a.X, b.Y-a.Y) }

	for _, c := range []struct {
		name string
		path []city.Point
	}{
		{"a straight path", []city.Point{{X: 0, Y: 0}, {X: 120, Y: 0}}},
		{"an L-shaped path", []city.Point{{X: 10, Y: 10}, {X: 70, Y: 10}, {X: 70, Y: 130}}},
	} {
		t.Run("for "+c.name, func(t *testing.T) {
			dashes := planDashes(c.path, on, off)
			require.NotEmpty(t, dashes, "precondition: there are dashes")
			total := 0.0
			for i := 1; i < len(c.path); i++ {
				total += length(c.path[i-1], c.path[i])
			}
			inked, longest := 0.0, 0.0
			for _, d := range dashes {
				span := 0.0
				for i := 1; i < len(d); i++ {
					span += length(d[i-1], d[i])
				}
				inked += span
				longest = math.Max(longest, span)
			}

			t.Run("it should ink about half the path's length", func(t *testing.T) {
				assert.InDelta(t, 0.5, inked/total, 0.05)
			})

			t.Run("it should make no dash longer than the dash", func(t *testing.T) {
				assert.LessOrEqual(t, longest, on+1e-9)
			})

			t.Run("it should start at the path's first point", func(t *testing.T) {
				assert.Equal(t, c.path[0], dashes[0][0])
			})
		})
	}
}
