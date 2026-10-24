package render

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 108: the compaction's trucks come off the street by the lot's
// driveway and leave by it, as everything that drives a road does.
func TestJobTrucksUseTheDriveway(t *testing.T) {
	at := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	var sessions []state.Session
	for i := 0; i < 5; i++ {
		sessions = append(sessions, state.Session{ID: fmt.Sprint("s", i), CWD: "/p/a", State: state.Working, StartedAt: at.Add(time.Duration(i) * time.Minute)})
	}
	c := city.Build(state.Snapshot{At: at, Sessions: sessions}, city.NewLayout())
	require.Len(t, c.Districts, 1, "precondition: one district")
	d := c.Districts[0]
	dw, ok := c.Driveway(d)
	require.True(t, ok, "precondition: the lot has a driveway")
	cam := city.NewCamera()
	cam.Projection = city.Isometric
	g := &Game{}
	has := func(path []city.Point, p city.Point) bool {
		for _, q := range path {
			if q == p {
				return true
			}
		}
		return false
	}

	for _, b := range d.Buildings {
		corner, sides := lotCorner(cam, b)
		for _, stop := range []struct {
			name string
			at   city.Point
		}{{"the skip", corner}, {"the pour", corner.Add(sides[0].Sub(corner).Scale(0.5))}} {
			t.Run(fmt.Sprintf("when a truck drives to %s at %s", stop.name, b.Session.ID), func(t *testing.T) {
				in, out := g.jobPaths(c, b, stop.at)
				require.NotEmpty(t, in)
				require.NotEmpty(t, out)

				t.Run("it should come in off the street at the lot's driveway", func(t *testing.T) {
					assert.True(t, has(in, dw.Cell.Center()))
				})

				t.Run("it should come in through the lot end", func(t *testing.T) {
					assert.True(t, has(in, dw.Lot))
				})

				t.Run("it should stop at its stop", func(t *testing.T) {
					assert.Equal(t, stop.at, in[len(in)-1])
				})

				t.Run("it should leave through the lot end", func(t *testing.T) {
					assert.True(t, has(out, dw.Lot))
				})

				t.Run("it should leave onto the street at the driveway", func(t *testing.T) {
					assert.True(t, has(out, dw.Cell.Center()))
				})

				t.Run("it should leave the way it came", func(t *testing.T) {
					assert.Equal(t, in[0], out[len(out)-1])
				})

				t.Run("it should not drive through any building in the lot", func(t *testing.T) {
					var through []city.Point
					for k := 1; k < len(in); k++ {
						for s := 0.0; s <= 1; s += 0.05 {
							p := in[k-1].Add(in[k].Sub(in[k-1]).Scale(s))
							for _, other := range d.Buildings {
								if other.Rect.Inset(city.Tile / 2).Contains(p) {
									through = append(through, p)
								}
							}
						}
					}
					assert.Empty(t, through)
				})
			})
		}
	}
}
