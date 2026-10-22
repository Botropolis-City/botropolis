package state_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 159: a city that opens while the daemon's first load runs shows
// how far it has got.
func TestLoadProgress(t *testing.T) {
	t.Run("when a loader has not loaded", func(t *testing.T) {
		t.Run("it should report nothing done of nothing", func(t *testing.T) {
			assert.Equal(t, state.LoadProgress{}, state.NewLoader(t.TempDir(), alive).LoadProgress())
		})
	})

	t.Run("when a load has read a home of one live session and three stopped ones", func(t *testing.T) {
		home := helpers.NewHome(t)
		at := now.Format(time.RFC3339Nano)
		for i := range 4 {
			sid := fmt.Sprintf("dddddddd-0000-0000-0000-00000000015%d", i)
			home.Transcript(sid, loaderCWD, helpers.UserPrompt(sid, loaderCWD, at))
			if i == 0 {
				home.Session(7150, sid, loaderCWD, "interactive", "idle")
			}
		}
		loader := state.NewLoader(home.Path, alive)
		_, err := loader.Load(now)
		require.NoError(t, err)
		got := loader.LoadProgress()

		t.Run("it should count every session it found, the live one once more for its record", func(t *testing.T) {
			assert.Equal(t, 5, got.Total)
		})

		t.Run("it should have done them all", func(t *testing.T) {
			assert.Equal(t, got.Total, got.Done)
		})
	})
}
