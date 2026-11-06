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

// Item 350: the card's face is one direct verb and three submenu
// buttons; a submenu button opens the action menu on its column, Enter
// runs the direct verb, and the numbers row folds the details.
func TestCardFace(t *testing.T) {
	const id = "00000000-0000-4000-8000-000000000350"
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.Local)
	selected := func(t *testing.T) *Drive {
		t.Helper()
		h := helpers.NewHome(t)
		cwd := "/home/user/work/harbour"
		ts := now.Add(-8 * 24 * time.Hour).UTC().Format(time.RFC3339Nano)
		h.Transcript(id, cwd, helpers.UserPrompt(id, cwd, ts), helpers.AssistantReply(id, "msg_1", ts), helpers.AITitle(id, "Boarded up a week"))
		d, err := NewDrive(DriveOptions{Home: h.Path, Now: now})
		require.NoError(t, err)
		require.NoError(t, d.Run(10))
		require.NoError(t, d.Click(id))
		require.NoError(t, d.Run(5))
		require.Equal(t, id, d.Shown().Selected)
		return d
	}
	labels := func(s Shown) []string {
		var out []string
		for _, b := range s.CardButtons {
			out = append(out, b.Label)
		}
		return out
	}

	t.Run("when a resting session's card is drawn", func(t *testing.T) {
		d := selected(t)
		shown := d.Shown()

		t.Run("it should show four buttons, the direct verb first", func(t *testing.T) {
			assert.Equal(t, []string{"resume", "Session ▾", "Project ▾", "Group ▾"}, labels(shown))
		})

		t.Run("and the direct button is clicked", func(t *testing.T) {
			require.NoError(t, d.Click("button:resume"))
			require.NoError(t, d.Run(5))

			t.Run("it should resume it", func(t *testing.T) {
				assert.Contains(t, d.Acted(), city.Action{Kind: city.ActionResume, SessionID: id})
			})
		})
	})

	t.Run("when Session ▾ is clicked", func(t *testing.T) {
		d := selected(t)
		button := d.Shown().CardButtons[1].Rect
		require.NoError(t, d.Click("button:Session ▾"))
		require.NoError(t, d.Run(3))
		shown := d.Shown()

		t.Run("it should open the menu on the Session column", func(t *testing.T) {
			assert.Equal(t, []any{true, ui.MenuFocus{Column: 0, Row: 0}}, []any{shown.Menu, shown.MenuFocus})
		})

		t.Run("it should open it under the button", func(t *testing.T) {
			assert.True(t, shown.MenuRect.Min.Y >= button.Max.Y && shown.MenuRect.Area() > 0)
		})

		t.Run("and p is then pressed", func(t *testing.T) {
			require.NoError(t, d.Press("p"))
			require.NoError(t, d.Run(5))

			t.Run("it should ask the actor to pack it", func(t *testing.T) {
				assert.Contains(t, d.Acted(), city.Action{Kind: city.ActionPack, SessionID: id})
			})

			t.Run("it should close the menu", func(t *testing.T) {
				assert.False(t, d.Shown().Menu)
			})
		})
	})

	t.Run("when a menu row is clicked", func(t *testing.T) {
		d := selected(t)
		require.NoError(t, d.Click("button:Project ▾"))
		require.NoError(t, d.Run(3))
		require.NoError(t, d.Click("menu:copy path"))
		require.NoError(t, d.Run(5))

		t.Run("it should run that row", func(t *testing.T) {
			assert.Contains(t, d.Acted(), city.Action{Kind: city.ActionCopyPath, Dir: "/home/user/work/harbour"})
		})
	})

	t.Run("when Session ▾ is hovered", func(t *testing.T) {
		d := selected(t)
		require.NoError(t, d.PointAt("button:Session ▾"))
		require.NoError(t, d.Run(3))

		t.Run("the card's hint should list the column's rows", func(t *testing.T) {
			assert.Equal(t, "Session: resume, read, generate summary, pack, demolish", d.Shown().CardHint)
		})
	})

	t.Run("when Enter is pressed in Attention", func(t *testing.T) {
		d := selected(t)
		require.NoError(t, d.Press("Enter"))
		require.NoError(t, d.Run(5))

		t.Run("it should resume it, as the direct verb", func(t *testing.T) {
			assert.Contains(t, d.Acted(), city.Action{Kind: city.ActionResume, SessionID: id})
		})
	})

	t.Run("when Enter is pressed in Cost", func(t *testing.T) {
		d := selected(t)
		require.NoError(t, d.Press("Digit3"))
		require.NoError(t, d.Run(3))
		require.Equal(t, "breakdown", labels(d.Shown())[0])
		require.NoError(t, d.Press("Enter"))
		require.NoError(t, d.Run(5))

		t.Run("it should open the breakdown, as the direct verb", func(t *testing.T) {
			assert.True(t, d.Shown().Breakdown)
		})

		t.Run("it should not resume it", func(t *testing.T) {
			assert.NotContains(t, d.Acted(), city.Action{Kind: city.ActionResume, SessionID: id})
		})
	})

	t.Run("when the numbers row is clicked", func(t *testing.T) {
		d := selected(t)
		require.False(t, d.Shown().Details)
		require.NoError(t, d.Click("numbers"))
		require.NoError(t, d.Run(3))

		t.Run("it should fold the details out", func(t *testing.T) {
			assert.True(t, d.Shown().Details)
		})

		t.Run("and clicked again", func(t *testing.T) {
			require.NoError(t, d.Click("numbers"))
			require.NoError(t, d.Run(3))

			t.Run("it should fold them back", func(t *testing.T) {
				assert.False(t, d.Shown().Details)
			})
		})
	})

	t.Run("when the numbers row is hovered", func(t *testing.T) {
		d := selected(t)
		require.NoError(t, d.PointAt("numbers"))
		require.NoError(t, d.Run(3))

		t.Run("the card's hint should say it folds the details", func(t *testing.T) {
			assert.Equal(t, city.ActionDetails.Describe(), d.Shown().CardHint)
		})
	})
}
