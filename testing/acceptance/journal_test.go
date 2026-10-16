package acceptance_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/render"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 86, slice 5: opening the map after a weekend shows where each
// piece of work was and what is next, on the first frame, with no input.
func TestJournalAfterAGap(t *testing.T) {
	friday := time.Date(2026, 10, 16, 17, 0, 0, 0, time.Local)
	monday := friday.Add(64 * time.Hour)
	home := helpers.NewHome(t)
	for _, id := range []string{"11111111-0000-0000-0000-000000000001", "22222222-0000-0000-0000-000000000002"} {
		home.Transcript(id, "/src/kiln",
			helpers.UserPrompt(id, "/src/kiln", friday.Add(-time.Hour).UTC().Format(time.RFC3339Nano)),
			helpers.AssistantReply(id, "msg_"+id[:8], friday.Add(-50*time.Minute).UTC().Format(time.RFC3339Nano)))
	}

	t.Run("when the map opens after a gap", func(t *testing.T) {
		d, err := render.NewDrive(render.DriveOptions{Home: home.Path, Now: monday, Seen: friday})
		require.NoError(t, err)

		t.Run("the journal should be drawn with no input", func(t *testing.T) {
			assert.NotEmpty(t, d.Shown().Journal)
		})

		t.Run("it should cover every session, grouped or not", func(t *testing.T) {
			n := 0
			for _, e := range d.Shown().Journal {
				n += len(e.Sessions)
			}
			assert.Equal(t, 2, n)
		})

		t.Run("it should have taken no steps", func(t *testing.T) {
			assert.Zero(t, d.Steps())
		})

		t.Run("each entry should say what is next", func(t *testing.T) {
			for _, e := range d.Shown().Journal {
				assert.NotEmpty(t, e.Next, e.Title)
			}
		})
	})

	t.Run("when the map has no record of being seen", func(t *testing.T) {
		d, err := render.NewDrive(render.DriveOptions{Home: home.Path, Now: monday})
		require.NoError(t, err)

		t.Run("it should open on the map", func(t *testing.T) {
			assert.Empty(t, d.Shown().Journal)
		})
	})
}
