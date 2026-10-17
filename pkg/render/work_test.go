package render

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// Item 89: a working building lights one floor, and each tool call runs
// a pulse up its face from that floor to the roof.
func TestBuildingWork(t *testing.T) {
	r := city.RectAt(100, 50, 60, 120) // the building's drawn rect on screen
	const floor = 0.4

	for _, level := range city.LivelinessLevels {
		t.Run("when the level is "+level.String(), func(t *testing.T) {
			t.Run("a mid-turn building should draw its working floor inside its own rect", func(t *testing.T) {
				band := workingFloorBand(r, floor)
				assert.True(t, band.Area() > 0 && r.Contains(band.Min) && r.Contains(band.Max))
			})
		})
	}

	t.Run("when a pulse starts", func(t *testing.T) {
		t.Run("it should be on the working floor", func(t *testing.T) {
			band := workingFloorBand(r, floor)
			assert.InDelta(t, band.Center().Y, pulseAt(r, floor, 0).Y, 1e-9)
		})
	})

	t.Run("when a pulse ends", func(t *testing.T) {
		t.Run("it should be at the roof", func(t *testing.T) {
			assert.InDelta(t, r.Min.Y, pulseAt(r, floor, 1).Y, 1e-9)
		})
	})

	t.Run("while a pulse climbs", func(t *testing.T) {
		t.Run("it should stay on the building's face", func(t *testing.T) {
			var off int
			for i := 0; i <= 20; i++ {
				if !r.Contains(pulseAt(r, floor, float64(i)/20)) {
					off++
				}
			}
			assert.Zero(t, off)
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
