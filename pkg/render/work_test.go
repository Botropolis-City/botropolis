package render

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// Item 89: a working building lights one floor round the faces toward
// the camera, and each tool call runs a pulse up its front corner from
// that floor to the roof.
func TestBuildingWork(t *testing.T) {
	// A footprint on screen, north, east, south, west, and a roof 60 px up.
	lot := [4]city.Point{{X: 100, Y: 80}, {X: 160, Y: 110}, {X: 100, Y: 140}, {X: 40, Y: 110}}
	const roofLift, floor = 60.0, 0.4
	front := lot[2]

	for _, level := range city.LivelinessLevels {
		t.Run("when the level is "+level.String(), func(t *testing.T) {
			t.Run("a mid-turn building should light its working floor on both faces toward the camera, at the floor's height", func(t *testing.T) {
				faces := workingFloor(lot, roofLift, floor)
				var wrong int
				for _, face := range faces {
					for _, p := range face {
						h := front.Y - p.Y
						if p.X < lot[3].X || p.X > lot[1].X || h < 0 || h > roofLift+lot[2].Y-lot[0].Y {
							wrong++
						}
					}
				}
				assert.Zero(t, wrong)
			})
		})
	}

	t.Run("when a pulse starts", func(t *testing.T) {
		t.Run("it should be on the working floor, at the front corner", func(t *testing.T) {
			assert.Equal(t, city.Point{X: front.X, Y: front.Y - floor*roofLift}, pulseAt(lot, roofLift, floor, 0))
		})
	})

	t.Run("when a pulse ends", func(t *testing.T) {
		t.Run("it should be at the roof, at the front corner", func(t *testing.T) {
			assert.Equal(t, city.Point{X: front.X, Y: front.Y - roofLift}, pulseAt(lot, roofLift, floor, 1))
		})
	})

	t.Run("when a drone is drawn at the fit zoom on a small building", func(t *testing.T) {
		t.Run("it should be at least 12 px wide", func(t *testing.T) {
			assert.GreaterOrEqual(t, droneWidth(20), 12.0)
		})
	})

	t.Run("when a drone is drawn at a normal zoom", func(t *testing.T) {
		t.Run("it should still be about one window wide", func(t *testing.T) {
			assert.InDelta(t, droneWindow*200, droneWidth(200), 1e-9)
		})
	})
}
