package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 323: past the drawn sea's edge the frame showed its clear colour,
// so the ground is cleared to the sea's own colour once there is a sea.
func TestTheGroundIsClearedToTheSea(t *testing.T) {
	start := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)

	t.Run("when the city has an island", func(t *testing.T) {
		sessions := []state.Session{{ID: "s", CWD: "/p/d", State: state.Working, Alive: true, StartedAt: start}}
		c := city.Build(state.Snapshot{At: start, Sessions: sessions}, city.NewLayout())
		require.Positive(t, c.Bounds().Area(), "precondition: the city has a plan")

		for _, light := range []struct {
			name  string
			night bool
			sea   any
		}{
			{"by day", false, dayShore.sea},
			{"by night", true, nightShore.sea},
		} {
			t.Run("and it is drawn "+light.name, func(t *testing.T) {
				c.Night = light.night

				t.Run("it should clear the ground to that light's sea colour", func(t *testing.T) {
					assert.Equal(t, light.sea, groundClear(c))
				})
			})
		}
	})

	t.Run("when the city has no plan", func(t *testing.T) {
		c := city.Build(state.Snapshot{At: start}, city.NewLayout())

		t.Run("it should clear the ground to the void", func(t *testing.T) {
			assert.Equal(t, colorVoid, groundClear(c))
		})
	})
}
