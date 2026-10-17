package render

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

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

		held := 0
		for i := 1; i < len(got); i++ {
			if got[i].at == got[i-1].at {
				held++
			}
		}

		t.Run("it should pause at a point before moving on", func(t *testing.T) {
			assert.Greater(t, held, samples/10)
		})

		// Aria, 2026-10-17: busy, not idle.
		t.Run("it should hold still for under half of the samples", func(t *testing.T) {
			assert.Less(t, held, samples/2)
		})

		t.Run("it should reach a new point at least once a second", func(t *testing.T) {
			arrivals := 0
			for i := 2; i < len(got); i++ {
				if got[i].at == got[i-1].at && got[i-1].at != got[i-2].at {
					arrivals++
				}
			}
			assert.GreaterOrEqual(t, float64(arrivals), samples*step)
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

	t.Run("when a drone is drawn on a building", func(t *testing.T) {
		t.Run("it should be about one window wide: a tenth to a fifth of its building's drawn width", func(t *testing.T) {
			assert.True(t, droneWindow >= 0.1 && droneWindow <= 0.2)
		})
	})

	// Aria, 2026-10-17: proper perspective. A drone stands in the one
	// depth sort on its shadow's ground point.
	t.Run("in the depth sort", func(t *testing.T) {
		cam := city.NewCamera()
		named := func(build func(rec func(string) func()) []drawable) []string {
			return namedOrder(cam, build)
		}
		own := building.Rect
		offFace := faceBase(cam, own, 0.5)
		taller := city.RectAt(own.Max.X+city.Tile, own.Max.Y+city.Tile, own.Width(), own.Height())
		sign := func(at city.Point) city.Rect { return city.RectAt(at.X-1, at.Y-1, 2, 2) }

		for _, c := range []struct {
			name  string
			build func(rec func(string) func()) []drawable
			want  []string
		}{
			{"when a drone works on the roof of its own building, it should be drawn after its building",
				func(rec func(string) func()) []drawable {
					return []drawable{droneAtDepth(cam, own.Center(), rec("drone")), footprintAt(cam, own, rec("building"))}
				}, []string{"building", "drone"}},
			{"when a drone works on a face toward the camera, it should be drawn after its building",
				func(rec func(string) func()) []drawable {
					return []drawable{droneAtDepth(cam, offFace, rec("drone")), footprintAt(cam, own, rec("building"))}
				}, []string{"building", "drone"}},
			{"when a taller building stands in front of the drone's building, it should be drawn before that building",
				func(rec func(string) func()) []drawable {
					return []drawable{footprintAt(cam, taller, rec("taller")), droneAtDepth(cam, own.Center(), rec("drone")), footprintAt(cam, own, rec("building"))}
				}, []string{"building", "drone", "taller"}},
			{"when a drone is behind a district sign, it should be drawn before the sign",
				func(rec func(string) func()) []drawable {
					at := city.Point{X: own.Max.X + city.Tile, Y: own.Max.Y + city.Tile}
					return []drawable{footprintAt(cam, sign(at), rec("sign")), droneAtDepth(cam, own.Center(), rec("drone"))}
				}, []string{"drone", "sign"}},
			{"when a drone is in front of a district sign, it should be drawn after the sign",
				func(rec func(string) func()) []drawable {
					at := city.Point{X: own.Min.X - city.Tile, Y: own.Min.Y - city.Tile}
					return []drawable{droneAtDepth(cam, own.Center(), rec("drone")), footprintAt(cam, sign(at), rec("sign"))}
				}, []string{"sign", "drone"}},
		} {
			t.Run(c.name, func(t *testing.T) {
				assert.Equal(t, c.want, named(c.build))
			})
		}
	})
}
