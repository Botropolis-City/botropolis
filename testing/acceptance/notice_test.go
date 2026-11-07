package acceptance_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/render"
)

// Item 352: what just happened shows on one plate top-centre under the
// strip, and as the stream's newest line.
func TestNoticePlate(t *testing.T) {
	home := stageDemo(t, "hamlet")
	const width, height = 1600, 900

	t.Run("when tab is pressed", func(t *testing.T) {
		d, err := render.NewDrive(render.DriveOptions{Home: home, Width: width, Height: height})
		require.NoError(t, err)
		require.NoError(t, d.Run(1))
		before := d.Shown()
		require.NoError(t, d.Press("tab"))
		require.NoError(t, d.Run(1))
		s := d.Shown()

		t.Run("it should show a notice", func(t *testing.T) {
			assert.NotEmpty(t, s.Notice)
		})

		t.Run("it should centre the plate on the window", func(t *testing.T) {
			assert.InDelta(t, width/2, s.NoticeRect.Center().X, 1)
		})

		t.Run("it should put the plate under the strip", func(t *testing.T) {
			assert.GreaterOrEqual(t, s.NoticeRect.Min.Y, s.StripBottom)
		})

		t.Run("it should show the same line newest in the stream", func(t *testing.T) {
			assert.Equal(t, s.Notice, s.StreamNewest)
		})

		t.Run("it should not move the map's bottom", func(t *testing.T) {
			assert.Equal(t, before.MapBottom, s.MapBottom)
		})
	})
}
