package render

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 85: the river's blue is not cut square where it meets the sea.
// mouthStep is the most any colour channel may change between two
// neighbouring texels along a mouth's centre line; the square cut it
// replaced stepped by 29.
const mouthStep = 8

func TestRiverMouthBlends(t *testing.T) {
	var sessions []state.Session
	for _, root := range []string{"/p/a", "/p/b", "/p/c"} {
		sessions = append(sessions, state.Session{ID: root, CWD: root, State: state.Working})
	}
	c := city.Build(state.Snapshot{Sessions: sessions}, city.NewLayout())
	island := c.Island()
	from, _, _, ok := c.RiverBand()
	require.True(t, ok)
	b, reach := c.Bounds(), island.Reach()

	for _, mouth := range []struct {
		name     string
		from, to float64
	}{
		{"north", b.Min.Y, reach.Min.Y - city.CellSize},
		{"south", b.Max.Y, reach.Max.Y + city.CellSize},
	} {
		t.Run("along the "+mouth.name+" mouth's centre line, from the plan's edge to the open sea", func(t *testing.T) {
			worst := 0.0
			dir := math.Copysign(1, mouth.to-mouth.from)
			prev := shorePaint(island.At(city.Point{X: from.X, Y: mouth.from}))
			for y := mouth.from + dir*shoreTexel; dir*(mouth.to-y) > 0; y += dir * shoreTexel {
				cur := shorePaint(island.At(city.Point{X: from.X, Y: y}))
				for _, d := range []float64{float64(cur.R) - float64(prev.R), float64(cur.G) - float64(prev.G), float64(cur.B) - float64(prev.B)} {
					worst = math.Max(worst, math.Abs(d))
				}
				prev = cur
			}

			t.Run("it should change colour by no more than a small step between neighbouring texels", func(t *testing.T) {
				assert.LessOrEqual(t, worst, float64(mouthStep))
			})
		})
	}
}
