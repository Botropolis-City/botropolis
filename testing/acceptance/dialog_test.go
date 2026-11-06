package acceptance_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/render"
)

// Item 352: demolishing asks in a centred dialog, and d d still answers
// it from the keyboard.
func TestDemolishDialog(t *testing.T) {
	home := stageDemo(t, "hamlet")
	const width, height = 1600, 900
	demolitions := func(d *render.Drive) int {
		n := 0
		for _, a := range d.Acted() {
			if a.Kind == city.ActionDemolish {
				n++
			}
		}
		return n
	}

	t.Run("when tab selects a session and d is pressed", func(t *testing.T) {
		d, err := render.NewDrive(render.DriveOptions{Home: home, Width: width, Height: height})
		require.NoError(t, err)
		require.NoError(t, d.Press("tab"))
		require.NoError(t, d.Press("d"))
		require.NoError(t, d.Run(1))
		s := d.Shown()

		t.Run("it should show the demolish dialog", func(t *testing.T) {
			assert.Contains(t, s.Dialog, "Demolish ")
		})

		t.Run("it should centre it in the window", func(t *testing.T) {
			assert.Equal(t, city.Point{X: width / 2, Y: height / 2}, s.DialogRect.Center())
		})

		t.Run("it should not have demolished", func(t *testing.T) {
			assert.Zero(t, demolitions(d))
		})

		t.Run("and Escape is pressed", func(t *testing.T) {
			require.NoError(t, d.Press("Escape"))
			require.NoError(t, d.Run(1))

			t.Run("it should take the dialog away", func(t *testing.T) {
				assert.Empty(t, d.Shown().Dialog)
			})

			t.Run("it should not have demolished", func(t *testing.T) {
				assert.Zero(t, demolitions(d))
			})
		})
	})

	t.Run("when d is pressed twice from the map", func(t *testing.T) {
		d, err := render.NewDrive(render.DriveOptions{Home: home, Width: width, Height: height})
		require.NoError(t, err)
		require.NoError(t, d.Press("tab"))
		require.NoError(t, d.Press("d"))
		require.NoError(t, d.Press("d"))
		require.NoError(t, d.Run(1))

		t.Run("it should ask the actor to demolish once", func(t *testing.T) {
			assert.Equal(t, 1, demolitions(d))
		})

		t.Run("it should leave no dialog up", func(t *testing.T) {
			assert.Empty(t, d.Shown().Dialog)
		})
	})
}
