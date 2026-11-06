package acceptance_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/render"
)

// Item 350, slice 3: a right-click on a building opens the action menu
// at the pointer, through the city's own mouse path.
func TestRightClickMenu(t *testing.T) {
	home := stageDemo(t, "hamlet")
	d, err := render.NewDrive(render.DriveOptions{Home: home, Width: 1280, Height: 800})
	require.NoError(t, err)
	_, err = d.Until(func(s render.Shown) bool { return s.Settled }, settleTicks)
	require.NoError(t, err)
	require.NotEmpty(t, d.Sessions())
	id := d.Sessions()[0]
	at, ok := d.Where(id)
	require.True(t, ok, "the session's building should be on screen")
	require.NoError(t, d.RightClickAt(at))
	_, err = d.Until(func(s render.Shown) bool { return s.Menu }, settleTicks)
	shown := d.Shown()

	t.Run("when a building is right-clicked", func(t *testing.T) {
		t.Run("it should open the menu", func(t *testing.T) {
			assert.NoError(t, err)
		})

		t.Run("it should select the building", func(t *testing.T) {
			assert.Equal(t, id, shown.Selected)
		})

		t.Run("it should open it at the pointer, or moved only to stay inside the window", func(t *testing.T) {
			r := shown.MenuRect
			assert.True(t, (r.Min.X == at.X || r.Max.X == 1280) && (r.Min.Y == at.Y || r.Max.Y == 800), "menu %v, pointer %v", r, at)
		})
	})
}
