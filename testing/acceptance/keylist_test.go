package acceptance_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/render"
)

// openKeyList opens the reference and its keys entry in a window of a
// size, and returns the drive with the list drawn.
func openKeyList(t *testing.T, home string, width, height int) *render.Drive {
	t.Helper()
	d, err := render.NewDrive(render.DriveOptions{Home: home, Width: width, Height: height})
	require.NoError(t, err)
	require.NoError(t, d.Press("?"))
	_, err = d.Until(func(s render.Shown) bool { return s.Help }, settleTicks)
	require.NoError(t, err)
	require.NoError(t, d.Click("term:keys"))
	_, err = d.Until(func(s render.Shown) bool { return s.KeyListRect.Area() > 0 }, settleTicks)
	require.NoError(t, err)
	return d
}

// Item 127: the key list fits the window or scrolls.
func TestKeyList(t *testing.T) {
	home := stageDemo(t, "hamlet")

	for _, size := range [][2]int{{700, 600}, {1024, 768}, {1600, 1000}, {1920, 1080}} {
		t.Run(fmt.Sprintf("when the key list is open in a %dx%d window", size[0], size[1]), func(t *testing.T) {
			d := openKeyList(t, home, size[0], size[1])
			window := city.RectAt(0, 0, float64(size[0]), float64(size[1]))

			t.Run("it should keep the list inside the window", func(t *testing.T) {
				assert.Equal(t, window, window.Union(d.Shown().KeyListRect))
			})

			t.Run("it should keep the list clear of the stream", func(t *testing.T) {
				s := d.Shown()
				assert.False(t, s.KeyListRect.Overlaps(s.StreamRect), "list %v, stream %v", s.KeyListRect, s.StreamRect)
			})

			t.Run("it should keep the list clear of the strip and the footer", func(t *testing.T) {
				s := d.Shown()
				room := city.Rect{Min: city.Point{Y: s.StripBottom}, Max: city.Point{X: window.Max.X, Y: s.FooterTop}}
				assert.Equal(t, room, room.Union(s.KeyListRect), "strip ends %v, footer starts %v", s.StripBottom, s.FooterTop)
			})
		})
	}

	t.Run("when the key list is open in a window tall enough for it", func(t *testing.T) {
		d := openKeyList(t, home, 1600, 1000)

		t.Run("it should not scroll", func(t *testing.T) {
			assert.Equal(t, 0, d.Shown().KeyListMax)
		})
	})

	t.Run("when the key list is open in a 700x600 window, too small for it", func(t *testing.T) {
		d := openKeyList(t, home, 700, 600)
		before := d.Shown()

		t.Run("it should be able to scroll", func(t *testing.T) {
			assert.Positive(t, before.KeyListMax)
		})

		t.Run("and the down arrow is pressed twice", func(t *testing.T) {
			require.NoError(t, d.Press("ArrowDown"))
			require.NoError(t, d.Press("ArrowDown"))
			require.NoError(t, d.Run(2))
			down := d.Shown()

			t.Run("it should scroll the list two rows", func(t *testing.T) {
				assert.Equal(t, 2, down.KeyListScrolled)
			})

			t.Run("and the wheel turns up over it", func(t *testing.T) {
				require.NoError(t, d.WheelAt(before.KeyListRect.Center(), 1))
				require.NoError(t, d.Run(2))
				up := d.Shown()

				t.Run("it should scroll the list back a row", func(t *testing.T) {
					assert.Equal(t, 1, up.KeyListScrolled)
				})

				t.Run("it should not zoom the map behind it", func(t *testing.T) {
					assert.Equal(t, before.Zoom, up.Zoom)
				})

				t.Run("and the up arrow is pressed", func(t *testing.T) {
					require.NoError(t, d.Press("ArrowUp"))
					require.NoError(t, d.Run(2))

					t.Run("it should scroll the list back to the top", func(t *testing.T) {
						assert.Equal(t, 0, d.Shown().KeyListScrolled)
					})

					t.Run("and the wheel turns down over it", func(t *testing.T) {
						require.NoError(t, d.WheelAt(before.KeyListRect.Center(), -1))
						require.NoError(t, d.Run(2))

						t.Run("it should scroll the list a row", func(t *testing.T) {
							assert.Equal(t, 1, d.Shown().KeyListScrolled)
						})
					})
				})
			})
		})
	})
}
