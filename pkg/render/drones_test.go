package render

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
)

// Item 89: a drone works on its building — it flits between points on
// the two faces toward the camera and the roof, pausing at each — and
// its shadow falls straight beneath it.
func TestDroneWork(t *testing.T) {
	const (
		roofLift = 40.0
		samples  = 400
		step     = 0.05 // seconds between samples
	)
	building := &city.Building{Rect: city.RectAt(5*city.Tile, 5*city.Tile, 2*city.Tile, 2*city.Tile)}
	working := func(seat int) city.Flight {
		return city.Flight{Seat: seat, Stage: city.Loitering, On: building, Over: building.Rect.Center(), Climb: 1, Fade: 1}
	}
	type sample struct{ at, shadow city.Point }
	run := func(cam *city.Camera, seat int) []sample {
		out := make([]sample, samples)
		for i := range out {
			out[i].at, out[i].shadow = droneAt(cam, working(seat), roofLift, float64(i)*step)
		}
		return out
	}
	depth := func(cam *city.Camera, p city.Point) float64 {
		q := cam.TurnPoint(p)
		return q.X + q.Y
	}
	// onRoof is a sample over the roof: its shadow inside the footprint
	// and the drone at or above the roof plane.
	onRoof := func(cam *city.Camera, s sample) bool {
		r := building.Rect
		inside := s.shadow.X >= r.Min.X && s.shadow.X <= r.Max.X && s.shadow.Y >= r.Min.Y && s.shadow.Y <= r.Max.Y
		return inside && s.at.Y <= cam.WorldToScreen(s.shadow).Y-roofLift+1e-9
	}
	// nearFront is how far a shadow is from the two edges toward the
	// camera, in world units.
	nearFront := func(cam *city.Camera, p city.Point) float64 {
		front, sides := frontCorner(cam, building.Rect)
		best := math.Inf(1)
		for _, side := range sides {
			best = math.Min(best, segmentDistance(p, front, side))
		}
		return best
	}

	for _, heading := range []int{0, 90, 180, 270} {
		t.Run(fmt.Sprintf("when a drone is working on a building and the camera faces %d", heading), func(t *testing.T) {
			cam := city.NewCamera()
			cam.Heading = heading
			got := run(cam, 0)

			t.Run("it should stay on, or within two units of, the faces toward the camera, or over the roof", func(t *testing.T) {
				var astray int
				for _, s := range got {
					if !onRoof(cam, s) && nearFront(cam, s.shadow) > 2 {
						astray++
					}
				}
				assert.Zero(t, astray)
			})

			t.Run("it should never sit behind its own building", func(t *testing.T) {
				var behind int
				for _, s := range got {
					if !onRoof(cam, s) && depth(cam, s.shadow) < depth(cam, building.Rect.Center()) {
						behind++
					}
				}
				assert.Zero(t, behind)
			})
		})
	}

	t.Run("when a drone is working on a building", func(t *testing.T) {
		cam := city.NewCamera()
		got := run(cam, 0)

		t.Run("it should visit at least two distinct points in a cycle", func(t *testing.T) {
			seen := map[city.Point]bool{}
			for _, s := range got {
				seen[s.at] = true
			}
			assert.GreaterOrEqual(t, len(seen), 2)
		})

		t.Run("it should pause at a point before moving on", func(t *testing.T) {
			var held int
			for i := 1; i < len(got); i++ {
				if got[i].at == got[i-1].at {
					held++
				}
			}
			assert.Greater(t, held, samples/4)
		})

		t.Run("it should cast its shadow straight beneath it", func(t *testing.T) {
			var off int
			for _, s := range got {
				if math.Abs(s.at.X-cam.WorldToScreen(s.shadow).X) > 1e-9 {
					off++
				}
			}
			assert.Zero(t, off)
		})
	})

	t.Run("when two drones work on one building", func(t *testing.T) {
		cam := city.NewCamera()
		a, b := run(cam, 0), run(cam, 1)

		t.Run("they should be at the same point in under a tenth of the samples", func(t *testing.T) {
			var together int
			for i := range a {
				if math.Hypot(a[i].at.X-b[i].at.X, a[i].at.Y-b[i].at.Y) < 1 {
					together++
				}
			}
			assert.Less(t, together, samples/10)
		})
	})

	t.Run("when a drone is on its pad", func(t *testing.T) {
		cam := city.NewCamera()
		f := working(0)
		f.Stage, f.Climb = city.TakingOff, 0
		at, _ := droneAt(cam, f, roofLift, 0)
		pad := cam.WorldToScreen(building.Rect.Center())

		t.Run("it should stand on its session's roof", func(t *testing.T) {
			assert.Equal(t, city.Point{X: pad.X, Y: pad.Y - roofLift}, at)
		})
	})

	t.Run("when a drone flies between districts", func(t *testing.T) {
		cam := city.NewCamera()
		over := city.Point{X: 200, Y: 200}
		at, shadow := droneAt(cam, city.Flight{Stage: city.Outbound, Over: over, Climb: 1, Fade: 1}, roofLift, 0)

		t.Run("it should fly above the roofline", func(t *testing.T) {
			assert.Less(t, at.Y, cam.WorldToScreen(over).Y-roofLift)
		})

		t.Run("it should cast its shadow straight beneath it", func(t *testing.T) {
			assert.InDelta(t, cam.WorldToScreen(shadow).X, at.X, 1e-9)
		})
	})

	t.Run("when a drone is drawn", func(t *testing.T) {
		t.Run("it should be drawn at under half its old size", func(t *testing.T) {
			assert.Less(t, droneShrink, 0.5)
		})
	})

	t.Run("when a drone works over a building and a taller one stands in front", func(t *testing.T) {
		cam := city.NewCamera()
		var drawn []string
		record := func(name string) func() { return func() { drawn = append(drawn, name) } }
		behind := city.RectAt(5*city.Tile, 5*city.Tile, city.Tile, city.Tile)
		front := city.RectAt(6*city.Tile, 6*city.Tile, city.Tile, city.Tile)
		draws := layered(cam,
			[]drawable{footprintAt(cam, front, record("front")), footprintAt(cam, behind, record("behind"))},
			[]func(){record("drone")})
		require.Len(t, draws, 3)
		for _, draw := range draws {
			draw()
		}

		t.Run("it should be drawn after both buildings", func(t *testing.T) {
			assert.Equal(t, []string{"behind", "front", "drone"}, drawn)
		})
	})
}
