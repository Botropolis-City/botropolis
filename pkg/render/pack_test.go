package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 278: packing reaches the actor from the card's button and from
// Shift+P, through the city's own clicks and keys.
func TestPackFromTheCity(t *testing.T) {
	const id = "00000000-0000-4000-8000-000000000278"
	now := time.Date(2026, 10, 26, 10, 0, 0, 0, time.Local)
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
		return d
	}
	packed := city.Action{Kind: city.ActionPack, SessionID: id}

	t.Run("when a shuttered building is selected", func(t *testing.T) {
		t.Run("and its card's pack button is clicked", func(t *testing.T) {
			d := selected(t)
			require.NoError(t, d.Click("button:pack"))
			require.NoError(t, d.Run(5))

			t.Run("it should ask the actor to pack it", func(t *testing.T) {
				assert.Contains(t, d.Acted(), packed)
			})
		})

		t.Run("and Shift+P is pressed", func(t *testing.T) {
			d := selected(t)
			require.NoError(t, d.Press("shift+p"))
			require.NoError(t, d.Run(5))

			t.Run("it should ask the actor to pack it", func(t *testing.T) {
				assert.Contains(t, d.Acted(), packed)
			})

			t.Run("it should say it is packing it up", func(t *testing.T) {
				assert.Equal(t, "packing Boarded up a week up", d.game.notice.Text(timeNow()))
			})
		})
	})
}
