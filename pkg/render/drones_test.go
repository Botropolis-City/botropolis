package render

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// Item 89: where a drone is drawn, and its shadow.
func TestDroneFlight(t *testing.T) {
	cam := city.NewCamera()
	over := city.Point{X: 200, Y: 200}
	roofLift := 40.0
	ground := cam.WorldToScreen(over)
	roofline := ground.Y - roofLift

	t.Run("when a drone is on its pad", func(t *testing.T) {
		at, _ := droneAt(cam, city.Flight{Over: over, Climb: 0, Fade: 1}, roofLift, 0)

		t.Run("it should stand on its session's roof", func(t *testing.T) {
			assert.Equal(t, city.Point{X: ground.X, Y: roofline}, at)
		})
	})

	t.Run("when a drone is loitering", func(t *testing.T) {
		at, shadow := droneAt(cam, city.Flight{Over: over, Climb: 1, Fade: 1, Stage: city.Loitering}, roofLift, 3)
		beneath := cam.WorldToScreen(shadow)

		t.Run("it should fly above the roofline", func(t *testing.T) {
			assert.Less(t, at.Y, roofline-droneCruise*cam.Zoom/2)
		})

		t.Run("it should cast its shadow on the ground straight beneath it", func(t *testing.T) {
			assert.InDelta(t, beneath.X, at.X, 1e-9)
		})
	})

	t.Run("over one loop of its loiter", func(t *testing.T) {
		var near, far = math.Inf(1), 0.0
		for i := 0; i < 360; i++ {
			seconds := float64(i) / 360 * loiterPeriod
			_, shadow := droneAt(cam, city.Flight{Over: over, Climb: 1, Fade: 1, Stage: city.Loitering}, roofLift, seconds)
			r := math.Hypot(shadow.X-over.X, shadow.Y-over.Y)
			near, far = math.Min(near, r), math.Max(far, r)
		}

		t.Run("it should stray from a perfect circle by more than a tenth of its radius", func(t *testing.T) {
			assert.Greater(t, far-near, loiterRadius*0.1)
		})

		t.Run("it should stay within half its radius of the circle", func(t *testing.T) {
			assert.True(t, near >= loiterRadius*0.5 && far <= loiterRadius*1.5)
		})
	})

	t.Run("when a drone loiters over a building and a taller one stands in front", func(t *testing.T) {
		var drawn []string
		record := func(name string) func() { return func() { drawn = append(drawn, name) } }
		behind := city.RectAt(5*city.Tile, 5*city.Tile, city.Tile, city.Tile)
		front := city.RectAt(6*city.Tile, 6*city.Tile, city.Tile, city.Tile)
		for _, item := range layered(cam,
			[]drawable{footprintAt(cam, front, record("front")), footprintAt(cam, behind, record("behind"))},
			[]func(){record("drone")}) {
			item()
		}

		t.Run("it should be drawn after both buildings", func(t *testing.T) {
			assert.Equal(t, []string{"behind", "front", "drone"}, drawn)
		})
	})
}
