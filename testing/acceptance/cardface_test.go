package acceptance_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/render"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 350: the card's face is four buttons, so a live grouped session's
// card stays no wider than its paragraph in every lens, in a window 960
// px wide or more, where the flat row of verbs used to wrap.
func TestCardFaceStaysNarrow(t *testing.T) {
	home := stageDemo(t, "team")
	th := ui.NewTheme(1)
	widest := ui.ParaWidth(th) + 4*th.Grid()
	lensKeys := []string{"Digit1", "Digit2", "Digit3", "Digit4", "Digit5", "Digit6"}

	for _, width := range []int{960, 1600} {
		t.Run(fmt.Sprintf("when the window is %d px wide", width), func(t *testing.T) {
			d, err := render.NewDrive(render.DriveOptions{Home: home, Width: width, Height: 800})
			require.NoError(t, err)
			_, err = d.Until(func(s render.Shown) bool { return s.Settled }, settleTicks)
			require.NoError(t, err)
			lead := ""
			for _, id := range d.Sessions() {
				if d.Title(id) == "Ask a teammate about kiln's reader" {
					lead = id
				}
			}
			require.NotEmpty(t, lead, "the team scenario should have its lead")
			require.NoError(t, d.Click(lead))
			_, err = d.Until(func(s render.Shown) bool { return s.Selected == lead && s.Settled }, settleTicks*3)
			require.NoError(t, err)

			for _, key := range lensKeys {
				t.Run("and the lens is "+key, func(t *testing.T) {
					require.NoError(t, d.Press(key))
					_, err := d.Until(func(s render.Shown) bool { return s.Settled && s.CardRect.Area() > 0 }, settleTicks*3)
					require.NoError(t, err)
					shown := d.Shown()

					t.Run("it should keep the card no wider than its paragraph and padding", func(t *testing.T) {
						assert.LessOrEqual(t, shown.CardRect.Width(), widest)
					})

					t.Run("it should lay the four buttons on one row", func(t *testing.T) {
						require.Len(t, shown.CardButtons, 4)
						assert.Equal(t, shown.CardButtons[0].Rect.Min.Y, shown.CardButtons[3].Rect.Min.Y)
					})
				})
			}
		})
	}
}
