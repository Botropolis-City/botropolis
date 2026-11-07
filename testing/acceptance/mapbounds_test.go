package acceptance_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/render"
)

// Item 353: there is no bar along the bottom, and the map has the
// window's height below the strip.
func TestMapBounds(t *testing.T) {
	home := stageDemo(t, "hamlet")

	for _, size := range [][2]int{{960, 720}, {1600, 1000}} {
		t.Run(fmt.Sprintf("when the city is drawn in a %dx%d window", size[0], size[1]), func(t *testing.T) {
			d, err := render.NewDrive(render.DriveOptions{Home: home, Width: size[0], Height: size[1]})
			require.NoError(t, err)
			require.NoError(t, d.Run(2))

			t.Run("it should give the map the window's full height", func(t *testing.T) {
				assert.Equal(t, float64(size[1]), d.Shown().MapBottom)
			})
		})
	}
}
