package city_test

import (
	"fmt"
	"testing"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
)

func TestCamera(t *testing.T) {
	t.Run("when the camera is new", func(t *testing.T) {
		cam := city.NewCamera()

		t.Run("it should be at unit zoom", func(t *testing.T) {
			assert.InDelta(t, 1.0, cam.Zoom, 1e-9)
		})

		t.Run("it should map the origin to the origin", func(t *testing.T) {
			assert.Equal(t, city.Point{}, cam.WorldToScreen(city.Point{}))
		})
	})

	t.Run("when the camera is panned and zoomed", func(t *testing.T) {
		cam := city.NewCamera()
		cam.Pan(city.Point{X: -30, Y: -10})
		cam.ZoomAt(city.Point{X: 0, Y: 0}, 2)

		t.Run("it should step up the ladder", func(t *testing.T) {
			assert.InDelta(t, 1.5, cam.Zoom, 1e-9)
		})

		t.Run("it should place a world point on the screen", func(t *testing.T) {
			assert.Equal(t, city.Point{X: 105, Y: 135}, cam.WorldToScreen(city.Point{X: 100, Y: 100}))
		})

		t.Run("it should take a screen point back to the world", func(t *testing.T) {
			assert.Equal(t, city.Point{X: 100, Y: 100}, cam.ScreenToWorld(city.Point{X: 105, Y: 135}))
		})
	})

	t.Run("when zooming at a point under the cursor", func(t *testing.T) {
		cam := city.NewCamera()
		cursor := city.Point{X: 400, Y: 300}
		before := cam.ScreenToWorld(cursor)
		cam.ZoomAt(cursor, 1.5)
		after := cam.ScreenToWorld(cursor)

		t.Run("it should keep that world point under the cursor", func(t *testing.T) {
			assert.InDelta(t, before.X, after.X, 1e-9)
			assert.InDelta(t, before.Y, after.Y, 1e-9)
		})
	})

	t.Run("when zooming far out", func(t *testing.T) {
		cam := city.NewCamera()
		for i := 0; i < 50; i++ {
			cam.ZoomAt(city.Point{}, 0.5)
		}

		t.Run("it should stop at the minimum zoom", func(t *testing.T) {
			assert.InDelta(t, city.MinZoom, cam.Zoom, 1e-9)
		})
	})

	t.Run("when zooming far in", func(t *testing.T) {
		cam := city.NewCamera()
		for i := 0; i < 50; i++ {
			cam.ZoomAt(city.Point{}, 2)
		}

		t.Run("it should stop at the maximum zoom", func(t *testing.T) {
			assert.InDelta(t, city.MaxZoom, cam.Zoom, 1e-9)
		})
	})

	t.Run("when the zoom sits between ladder steps", func(t *testing.T) {
		cam := city.NewCamera()
		cam.Zoom = 0.6

		t.Run("and the wheel turns in", func(t *testing.T) {
			in := *cam
			in.ZoomAt(city.Point{}, 2)

			t.Run("it should land on the next step up", func(t *testing.T) {
				assert.InDelta(t, 0.75, in.Zoom, 1e-9)
			})
		})

		t.Run("and the wheel turns out", func(t *testing.T) {
			out := *cam
			out.ZoomAt(city.Point{}, 0.5)

			t.Run("it should land on the next step down", func(t *testing.T) {
				assert.InDelta(t, 0.5, out.Zoom, 1e-9)
			})
		})
	})

	t.Run("when a fit falls between ladder steps", func(t *testing.T) {
		cam := city.NewCamera()
		cam.FitWithInsets(city.RectAt(0, 0, 1000, 1000), 1100, 1100, city.Insets{})

		t.Run("it should snap down to the step below", func(t *testing.T) {
			assert.InDelta(t, 1, cam.Zoom, 1e-9)
		})
	})

	t.Run("when the camera is fitted to a city", func(t *testing.T) {
		c := build(t, city.NewLayout(), session("a", cinders, state.Working), session("b", botropolis, state.Working))
		cam := city.NewCamera()
		cam.Fit(c.Bounds(), 800, 600)

		t.Run("it should bring the whole city on screen", func(t *testing.T) {
			min := cam.WorldToScreen(c.Bounds().Min)
			max := cam.WorldToScreen(c.Bounds().Max)
			assert.GreaterOrEqual(t, min.X, 0.0)
			assert.GreaterOrEqual(t, min.Y, 0.0)
			assert.LessOrEqual(t, max.X, 800.0)
			assert.LessOrEqual(t, max.Y, 600.0)
		})

		t.Run("it should centre it", func(t *testing.T) {
			centre := cam.WorldToScreen(c.Bounds().Center())
			assert.InDelta(t, 400, centre.X, 1e-6)
			assert.InDelta(t, (600-city.FitFooter)/2, centre.Y, 1e-6)
		})
	})

	t.Run("when the camera is fitted with a left inset", func(t *testing.T) {
		c := build(t, city.NewLayout(), session("a", cinders, state.Working))
		cam := city.NewCamera()
		cam.FitWithInsets(c.Bounds(), 800, 600, city.Insets{Left: 150, Bottom: city.FitFooter})

		t.Run("it should keep the city right of the inset", func(t *testing.T) {
			assert.GreaterOrEqual(t, cam.WorldToScreen(c.Bounds().Min).X, 150.0)
		})

		t.Run("it should centre it in the remaining width", func(t *testing.T) {
			assert.InDelta(t, 150+(800-150)/2.0, cam.WorldToScreen(c.Bounds().Center()).X, 1e-6)
		})
	})

	t.Run("when the city is far taller than a short window", func(t *testing.T) {
		var sessions []state.Session
		for i := 0; i < 60; i++ {
			sessions = append(sessions, session(fmt.Sprintf("s%02d", i), fmt.Sprintf("/p/%02d", i), state.Working))
		}
		c := build(t, city.NewLayout(), sessions...)
		cam := city.NewCamera()
		cam.FitWithInsets(c.Bounds(), 1200, 300, city.Insets{Bottom: city.FitFooter})

		t.Run("it should zoom out past the wheel floor to fit", func(t *testing.T) {
			assert.Less(t, cam.Zoom, city.MinZoom)
			assert.LessOrEqual(t, cam.WorldToScreen(c.Bounds().Max).Y, 300-city.FitFooter)
			assert.GreaterOrEqual(t, cam.WorldToScreen(c.Bounds().Min).Y, 0.0)
		})
	})

	t.Run("when an empty city is fitted", func(t *testing.T) {
		cam := city.NewCamera()
		cam.Fit(city.Rect{}, 800, 600)

		t.Run("it should stay at a usable zoom", func(t *testing.T) {
			assert.InDelta(t, 1.0, cam.Zoom, 1e-9)
		})
	})
}

func TestIsometricCamera(t *testing.T) {
	t.Run("when the camera is isometric", func(t *testing.T) {
		cam := city.NewCamera()
		cam.Projection = city.Isometric

		t.Run("it should put a world point where the projection says", func(t *testing.T) {
			assert.Equal(t, city.Isometric.Apply(city.Point{X: 40, Y: 10}), cam.WorldToScreen(city.Point{X: 40, Y: 10}))
		})

		t.Run("it should take a screen point back to the same world point", func(t *testing.T) {
			w := city.Point{X: 40, Y: 10}
			back := cam.ScreenToWorld(cam.WorldToScreen(w))
			assert.InDelta(t, w.X, back.X, 1e-9)
			assert.InDelta(t, w.Y, back.Y, 1e-9)
		})

		t.Run("and it zooms at a point under the cursor", func(t *testing.T) {
			zoomed := *cam
			zoomed.Pan(city.Point{X: 300, Y: 200})
			cursor := city.Point{X: 400, Y: 300}
			before := zoomed.ScreenToWorld(cursor)
			zoomed.ZoomAt(cursor, 2)
			after := zoomed.ScreenToWorld(cursor)

			t.Run("it should keep that world point under the cursor", func(t *testing.T) {
				assert.InDelta(t, before.X, after.X, 1e-9)
				assert.InDelta(t, before.Y, after.Y, 1e-9)
			})
		})

		t.Run("and it is fitted to a square", func(t *testing.T) {
			r := city.RectAt(0, 0, 400, 400)
			cam.FitWithInsets(r, 800, 600, city.Insets{})

			t.Run("it should keep every projected corner on screen", func(t *testing.T) {
				for _, corner := range city.Isometric.Corners(r) {
					p := cam.WorldToScreen(city.Isometric.Invert(corner))
					assert.GreaterOrEqual(t, p.X, 0.0)
					assert.LessOrEqual(t, p.X, 800.0)
					assert.GreaterOrEqual(t, p.Y, 0.0)
					assert.LessOrEqual(t, p.Y, 600.0)
				}
			})
		})
	})
}

func TestCameraHeading(t *testing.T) {
	t.Run("when the camera is turned a quarter", func(t *testing.T) {
		cam := city.NewCamera()
		cam.Projection = city.Isometric
		cam.Zoom = 2
		cam.Offset = city.Point{X: 300, Y: 200}
		pivot := city.Point{X: 400, Y: 300}
		under := cam.ScreenToWorld(pivot)
		cam.Turn(1, pivot)

		t.Run("it should face 90 degrees", func(t *testing.T) {
			assert.Equal(t, 90, cam.Heading)
		})

		t.Run("it should keep the world point under the pivot", func(t *testing.T) {
			after := cam.ScreenToWorld(pivot)
			assert.InDelta(t, under.X, after.X, 1e-6)
			assert.InDelta(t, under.Y, after.Y, 1e-6)
		})

		t.Run("it should round trip a point through the turned projection", func(t *testing.T) {
			p := city.Point{X: 123, Y: 45}
			back := cam.ScreenToWorld(cam.WorldToScreen(p))
			assert.InDelta(t, p.X, back.X, 1e-6)
			assert.InDelta(t, p.Y, back.Y, 1e-6)
		})

		t.Run("it should order depth by the turned axes", func(t *testing.T) {
			// At heading 90 the world's -y side is nearest.
			assert.Greater(t, cam.Depth(city.Point{X: 0, Y: -100}), cam.Depth(city.Point{X: 0, Y: 100}))
		})
	})

	t.Run("when the camera turns four quarters", func(t *testing.T) {
		cam := city.NewCamera()
		cam.Projection = city.Isometric
		before := *cam
		for i := 0; i < 4; i++ {
			cam.Turn(1, city.Point{X: 100, Y: 100})
		}

		t.Run("it should face home again", func(t *testing.T) {
			assert.Equal(t, 0, cam.Heading)
		})

		t.Run("it should be back where it started", func(t *testing.T) {
			assert.InDelta(t, before.Offset.X, cam.Offset.X, 1e-6)
			assert.InDelta(t, before.Offset.Y, cam.Offset.Y, 1e-6)
		})
	})

	t.Run("when the camera faces home", func(t *testing.T) {
		cam := city.NewCamera()
		cam.Projection = city.Isometric

		t.Run("it should project as the plain projection does", func(t *testing.T) {
			p := city.Point{X: 48, Y: 96}
			assert.Equal(t, city.Isometric.Apply(p), cam.Project(p))
		})
	})
}

// headings is every quarter turn the camera can face.
var headings = []int{0, 90, 180, 270}

func TestDepthOf(t *testing.T) {
	// Two cells side by side, and a piece spanning both, laid out on
	// the grid the plan uses.
	west := city.RectAt(0, 0, city.CellSize, city.CellSize)
	east := city.RectAt(city.CellSize, 0, city.CellSize, city.CellSize)
	south := city.RectAt(0, city.CellSize, city.CellSize, city.CellSize)
	wide := city.RectAt(0, 0, 2*city.CellSize, city.CellSize)

	t.Run("when a footprint is one cell", func(t *testing.T) {
		cam := city.NewCamera()

		t.Run("it should take the depth of its back corner", func(t *testing.T) {
			assert.InDelta(t, cam.Depth(west.Min), cam.DepthOf(west), 1e-9)
		})
	})

	t.Run("when one cell is behind another at every heading", func(t *testing.T) {
		cam := city.NewCamera()

		t.Run("it should order the far one first whichever way the camera faces", func(t *testing.T) {
			for _, heading := range headings {
				cam.Heading = heading
				near, far := east, west
				if cam.DepthOf(east) < cam.DepthOf(west) {
					near, far = west, east
				}
				assert.Less(t, cam.DepthOf(far), cam.DepthOf(near), "heading %d", heading)
			}
		})
	})

	t.Run("when a piece spans two cells and a neighbour sits beside it", func(t *testing.T) {
		cam := city.NewCamera()

		t.Run("it should not tie with the neighbour, as one point does", func(t *testing.T) {
			assert.Equal(t, cam.Depth(wide.Max), cam.Depth(south.Max), "the near corners do tie")
			assert.NotEqual(t, cam.DepthOf(wide), cam.DepthOf(south))
		})

		t.Run("it should be sorted by where it begins, so the neighbour draws over it", func(t *testing.T) {
			assert.Less(t, cam.DepthOf(wide), cam.DepthOf(south))
		})
	})

	t.Run("when the camera turns", func(t *testing.T) {
		cam := city.NewCamera()
		cam.Heading = 0
		front := cam.DepthOf(south)
		cam.Heading = 180
		back := cam.DepthOf(south)

		t.Run("it should follow the heading rather than the world", func(t *testing.T) {
			assert.NotEqual(t, front, back)
		})
	})

	t.Run("when a footprint is a single point", func(t *testing.T) {
		cam := city.NewCamera()
		dot := city.Rect{Min: city.Point{X: 12, Y: 20}, Max: city.Point{X: 12, Y: 20}}

		t.Run("it should be the depth of that point", func(t *testing.T) {
			assert.InDelta(t, cam.Depth(dot.Min), cam.DepthOf(dot), 1e-9)
		})
	})
}
