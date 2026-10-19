package acceptance_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/render"
)

// openSidebar opens the sidebar over a city with more projects than fit
// a short window, and returns the drive with it drawn.
func openSidebar(t *testing.T) *render.Drive {
	t.Helper()
	d, err := render.NewDrive(render.DriveOptions{Home: stageDemo(t, "metropolis"), Width: 1024, Height: 900})
	require.NoError(t, err)
	require.NoError(t, d.Press("b"))
	_, err = d.Until(func(s render.Shown) bool { return s.SidebarRect.Area() > 0 }, settleTicks)
	require.NoError(t, err)
	_, err = d.Until(func(s render.Shown) bool { return s.Settled }, settleTicks)
	require.NoError(t, err)
	require.Positive(t, d.Shown().SidebarMax, "the sidebar should have more rows than fit")
	return d
}

// Item 136: the sidebar scrolls, and keeps the selection's row in view.
func TestSidebarScrolls(t *testing.T) {
	t.Run("when the sidebar has more rows than fit", func(t *testing.T) {
		t.Run("and page down is pressed", func(t *testing.T) {
			d := openSidebar(t)
			before := d.Shown()
			require.NoError(t, d.Press("PageDown"))
			require.NoError(t, d.Run(2))
			after := d.Shown()

			t.Run("it should scroll the sidebar", func(t *testing.T) {
				assert.Positive(t, after.SidebarScrolled)
			})

			t.Run("it should leave the camera where it was", func(t *testing.T) {
				assert.Equal(t, before.Camera, after.Camera)
			})

			t.Run("and page up is pressed", func(t *testing.T) {
				require.NoError(t, d.Press("PageUp"))
				require.NoError(t, d.Run(2))

				t.Run("it should scroll back to the top", func(t *testing.T) {
					assert.Equal(t, 0, d.Shown().SidebarScrolled)
				})
			})
		})

		t.Run("and end is pressed", func(t *testing.T) {
			d := openSidebar(t)
			require.NoError(t, d.Press("End"))
			require.NoError(t, d.Run(2))
			end := d.Shown()

			t.Run("it should scroll to the last row", func(t *testing.T) {
				assert.Equal(t, end.SidebarMax, end.SidebarScrolled)
			})

			t.Run("and home is pressed", func(t *testing.T) {
				require.NoError(t, d.Press("Home"))
				require.NoError(t, d.Run(2))

				t.Run("it should scroll to the first row", func(t *testing.T) {
					assert.Equal(t, 0, d.Shown().SidebarScrolled)
				})
			})
		})

		t.Run("and the wheel turns down over it", func(t *testing.T) {
			d := openSidebar(t)
			require.NoError(t, d.WheelAt(d.Shown().SidebarRect.Center(), -2))
			require.NoError(t, d.Run(2))

			t.Run("it should scroll the sidebar two rows", func(t *testing.T) {
				assert.Equal(t, 2, d.Shown().SidebarScrolled)
			})
		})

		t.Run("and it is scrolled to the end and the wheel turns up over it", func(t *testing.T) {
			d := openSidebar(t)
			require.NoError(t, d.Press("End"))
			require.NoError(t, d.Run(2))
			before := d.Shown()
			require.NoError(t, d.WheelAt(before.SidebarRect.Center(), 2))
			require.NoError(t, d.Run(2))
			_, err := d.Until(func(s render.Shown) bool { return s.Settled }, settleTicks)
			require.NoError(t, err)
			after := d.Shown()

			t.Run("it should scroll the sidebar back two rows", func(t *testing.T) {
				assert.Equal(t, before.SidebarMax-2, after.SidebarScrolled)
			})

			t.Run("it should not zoom the map behind it", func(t *testing.T) {
				assert.Equal(t, before.Zoom, after.Zoom)
			})
		})

		t.Run("and the up arrow is held", func(t *testing.T) {
			d := openSidebar(t)
			before := d.Shown()
			require.NoError(t, d.Press("ArrowUp"))
			require.NoError(t, d.Run(2))
			after := d.Shown()

			t.Run("it should still pan the map", func(t *testing.T) {
				assert.NotEqual(t, before.Camera, after.Camera)
			})

			t.Run("it should not scroll the sidebar", func(t *testing.T) {
				assert.Equal(t, 0, after.SidebarScrolled)
			})
		})

		t.Run("and tab walks the sessions that need you", func(t *testing.T) {
			d := openSidebar(t)
			var missing []string
			for range 4 {
				require.NoError(t, d.Press("Tab"))
				require.NoError(t, d.Run(3))
				s := d.Shown()
				if s.Selected == "" || !slices.Contains(s.SidebarSessions, s.Selected) {
					missing = append(missing, s.Selected)
				}
			}

			t.Run("it should keep each selected session's row in view", func(t *testing.T) {
				assert.Empty(t, missing)
			})
		})
	})
}
