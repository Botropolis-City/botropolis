package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 351: the menu reads as its button's own: it opens under the
// button over the card, the button stays pressed with its chevron up
// and the column's heading lit, and a right-click still opens at the
// pointer.
func TestMenuAttached(t *testing.T) {
	const id = "00000000-0000-4000-8000-000000000351"
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.Local)
	selected := func(t *testing.T) *Drive {
		t.Helper()
		h := helpers.NewHome(t)
		cwd := "/home/user/work/harbour"
		ts := now.Add(-8 * 24 * time.Hour).UTC().Format(time.RFC3339Nano)
		h.Transcript(id, cwd, helpers.UserPrompt(id, cwd, ts), helpers.AssistantReply(id, "msg_1", ts), helpers.AITitle(id, "Boarded up a week"))
		d, err := NewDrive(DriveOptions{Home: h.Path, Now: now, Width: 1280, Height: 1100})
		require.NoError(t, err)
		require.NoError(t, d.Run(10))
		require.NoError(t, d.Click(id))
		require.NoError(t, d.Run(5))
		require.Equal(t, id, d.Shown().Selected)
		require.NoError(t, d.PointAtPoint(city.Point{X: 1, Y: 1}))
		require.NoError(t, d.Run(2))
		return d
	}
	button := func(s Shown, i int) ShownButton {
		require.Greater(t, len(s.CardButtons), i)
		return s.CardButtons[i]
	}
	grid := ui.NewTheme(1).Grid()

	for i, col := range city.VerbColumns {
		t.Run("when "+col.Title+" "+city.MenuMark+" is clicked", func(t *testing.T) {
			d := selected(t)
			before := d.Shown()
			opener := button(before, i+1).Rect
			require.NoError(t, d.Click("button:"+col.Title+" "+city.MenuMark))
			require.NoError(t, d.Run(3))
			shown := d.Shown()
			require.True(t, shown.Menu)

			t.Run("it should open with its left edge on the button's, or as near as the window's right edge lets it", func(t *testing.T) {
				assert.Equal(t, min(opener.Min.X, 1280-shown.MenuRect.Width()), shown.MenuRect.Min.X)
			})

			t.Run("it should open a grid under the button, over the card", func(t *testing.T) {
				assert.True(t, shown.MenuRect.Min.Y == opener.Max.Y+grid && shown.MenuRect.Min.Y < before.CardRect.Max.Y, "menu %v, button %v, card %v", shown.MenuRect, opener, before.CardRect)
			})

			t.Run("it should point its notch at the button", func(t *testing.T) {
				assert.True(t, shown.MenuNotch)
			})

			t.Run("it should show the button pressed with its chevron up", func(t *testing.T) {
				assert.Equal(t, ShownButton{Label: col.Title + " " + city.MenuMarkOpen, Rect: button(shown, i+1).Rect, Pressed: true}, button(shown, i+1))
			})

			t.Run("it should light the column's heading", func(t *testing.T) {
				assert.Equal(t, i, shown.MenuLit)
			})

			t.Run("and the menu is closed", func(t *testing.T) {
				require.NoError(t, d.Press("escape"))
				require.NoError(t, d.Run(3))
				after := button(d.Shown(), i+1)

				t.Run("it should let the button up, its chevron down", func(t *testing.T) {
					assert.Equal(t, []any{col.Title + " " + city.MenuMark, false}, []any{after.Label, after.Pressed})
				})
			})
		})
	}

	t.Run("when space is pressed with a session selected", func(t *testing.T) {
		d := selected(t)
		opener := button(d.Shown(), 1).Rect
		require.NoError(t, d.Press("space"))
		require.NoError(t, d.Run(3))
		shown := d.Shown()

		t.Run("it should open under the Session button", func(t *testing.T) {
			assert.Equal(t, city.Point{X: opener.Min.X, Y: opener.Max.Y + grid}, shown.MenuRect.Min)
		})

		t.Run("it should show Session pressed", func(t *testing.T) {
			assert.True(t, button(shown, 1).Pressed)
		})

		t.Run("it should point its notch at it", func(t *testing.T) {
			assert.True(t, shown.MenuNotch)
		})
	})

	t.Run("when the building is right-clicked", func(t *testing.T) {
		d := selected(t)
		at, ok := d.Where(id)
		require.True(t, ok)
		require.NoError(t, d.RightClickAt(at))
		require.NoError(t, d.Run(3))
		shown := d.Shown()
		require.True(t, shown.Menu)

		t.Run("it should lay no notch", func(t *testing.T) {
			assert.False(t, shown.MenuNotch)
		})

		t.Run("it should press no button", func(t *testing.T) {
			for _, b := range shown.CardButtons {
				assert.False(t, b.Pressed, b.Label)
			}
		})

		t.Run("it should light no heading", func(t *testing.T) {
			assert.Equal(t, -1, shown.MenuLit)
		})
	})
}
