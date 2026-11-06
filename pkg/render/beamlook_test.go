package render

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
)

// Item 347: a tower's lines light with the tower, and the other lines
// fade with the rest of the city.
func TestBeamLook(t *testing.T) {
	gc := newGroundCity(t)
	beams := gc.c.Beams()
	var files, search city.Beam
	for _, b := range beams {
		switch b.Tower.Server.Name {
		case "files":
			files = b
		case "search":
			search = b
		}
	}
	require.NotNil(t, files.Tower)
	require.NotNil(t, search.Tower)
	base := color.NRGBA{0x10, 0x20, 0x30, 0xff}
	alpha := float64(0xff) * recededAlpha
	faded := color.NRGBA{0x10, 0x20, 0x30, uint8(alpha)}
	none := beamStroke{}

	t.Run("when the highlight is up on a tower", func(t *testing.T) {
		gc.scene.SetHover(city.Hit{Tower: files.Tower})
		lit := gc.scene.Highlight()
		require.True(t, lit.Active)

		t.Run("its beam should keep its colour and draw the lit line over it", func(t *testing.T) {
			col, stroke := beamLook(lit, city.Hit{}, files, base)
			assert.Equal(t, base, col)
			assert.Equal(t, litStroke, stroke)
		})

		t.Run("another tower's beam should draw at the faded alpha, with no line over it", func(t *testing.T) {
			col, stroke := beamLook(lit, city.Hit{}, search, base)
			assert.Equal(t, faded, col)
			assert.Equal(t, none, stroke)
		})
	})

	t.Run("when a beam itself is under the pointer with no highlight up", func(t *testing.T) {
		gc.scene.SetHover(city.Hit{Beam: &files})
		lit := gc.scene.Highlight()
		require.False(t, lit.Active)

		t.Run("that beam should draw the lit line over it", func(t *testing.T) {
			col, stroke := beamLook(lit, city.Hit{Beam: &files}, files, base)
			assert.Equal(t, base, col)
			assert.Equal(t, litStroke, stroke)
		})

		t.Run("the other beams should keep their colour", func(t *testing.T) {
			col, stroke := beamLook(lit, city.Hit{Beam: &files}, search, base)
			assert.Equal(t, base, col)
			assert.Equal(t, none, stroke)
		})
	})

	t.Run("when a car's highlight recedes the ground", func(t *testing.T) {
		gc.scene.SetHover(city.Hit{Car: &city.CarHit{Road: gc.c.Streets[0].Road}})
		lit := gc.scene.Highlight()
		require.True(t, lit.RecedesGround())

		t.Run("no beam should be faded here, since the fader fades everything at once", func(t *testing.T) {
			col, stroke := beamLook(lit, city.Hit{}, search, base)
			assert.Equal(t, base, col)
			assert.Equal(t, none, stroke)
		})
	})

	// Aria, on the first frame: the lit lines were too thin and faint to
	// pick out from the faded ones at the fit zoom.
	t.Run("when a lit beam is drawn", func(t *testing.T) {
		t.Run("its line should be wider than the pointed-at line used to be", func(t *testing.T) {
			assert.Greater(t, litStroke.Width, float32(2))
		})

		t.Run("its line should be brighter than the highlight colour", func(t *testing.T) {
			assert.Greater(t, litStroke.Col.A, colorHighlight.A)
		})

		t.Run("its line should keep a width on screen at any zoom", func(t *testing.T) {
			assert.GreaterOrEqual(t, float64(litBeamMinPx), 3.0)
		})
	})
}
