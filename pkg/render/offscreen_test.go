package render

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/demo"
)

// manyDistricts is a made-up history of 300 sessions in 200 districts,
// settled at full zoom on a 1600×900 window.
func manyDistricts(t testing.TB) *Drive {
	t.Helper()
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.Local)
	history, err := demo.SynthHistory(270, 300, 200, now)
	require.NoError(t, err)
	home := t.TempDir()
	_, err = demo.WriteHome(home, history, "2006-01-02T15:04:05.000Z")
	require.NoError(t, err)
	d, err := NewDrive(DriveOptions{Home: home, Now: now, Width: 1600, Height: 900})
	require.NoError(t, err)
	require.NoError(t, d.Run(30))
	d.game.scene.Wheel(city.Point{X: 800, Y: 450}, 40)
	require.NoError(t, d.Run(120))
	return d
}

func TestDriveCullsWhatIsOffScreen(t *testing.T) {
	t.Run("when a city of 200 districts has settled at full zoom", func(t *testing.T) {
		d := manyDistricts(t)
		c := d.game.scene.City()
		standing := len(c.Trees) + len(c.Lamps) + len(c.Buildings()) + len(c.Districts)

		t.Run("it should sort fewer drawables than the city has trees, lamps, buildings and signs", func(t *testing.T) {
			assert.Less(t, d.game.sorted, standing/2)
		})

		t.Run("it should lay out fewer labels than the city has districts, since most of their plates cannot reach the screen", func(t *testing.T) {
			assert.Less(t, d.game.laidOut, len(c.Districts)/2)
		})
	})
}

// Item 270: a city of 900 districts sorted every tree, lamp, sign and
// monument it has each frame, on screen or not. What stands where it
// cannot be seen is left out of the sort.
func TestOnScreen(t *testing.T) {
	const width, height = 1600.0, 900.0
	frame := city.RectAt(0, 0, width, height)
	for _, heading := range []int{0, 90, 180, 270} {
		t.Run(fmt.Sprintf("when the camera faces %d degrees", heading), func(t *testing.T) {
			cam := city.NewCamera()
			cam.Projection, cam.Heading, cam.Offset = city.Isometric, heading, city.Point{X: 700, Y: 300}
			kept := func(at city.Point) bool {
				items := []drawable{standingAt(cam, cam.ScreenToWorld(at), func() {})}
				return len(onScreen(cam, items, frame)) == 1
			}
			for _, c := range []struct {
				name string
				at   city.Point
				want bool
			}{
				{"and a drawable stands in the middle of the screen, it should be kept", city.Point{X: width / 2, Y: height / 2}, true},
				{"and a drawable stands a screen's width to the left, it should be dropped", city.Point{X: -width, Y: height / 2}, false},
				{"and a drawable stands a screen's width to the right, it should be dropped", city.Point{X: 2 * width, Y: height / 2}, false},
				{"and a drawable stands above the top edge, it should be dropped", city.Point{X: width / 2, Y: -height / 2}, false},
				{"and a drawable's footprint is just below the bottom edge, as a tall building rising into view, it should be kept", city.Point{X: width / 2, Y: height + 60}, true},
				{"and a drawable stands two screens below, it should be dropped", city.Point{X: width / 2, Y: 3 * height}, false},
			} {
				t.Run(c.name, func(t *testing.T) {
					assert.Equal(t, c.want, kept(c.at))
				})
			}
		})
	}
}
