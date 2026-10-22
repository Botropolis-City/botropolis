package state_test

import (
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoaderGroupsAfterAPrompt(t *testing.T) {
	t.Run("when sessions are grouped from the prompt history", func(t *testing.T) {
		const sessions = 30
		home := helpers.NewHome(t)
		history := helpers.SyntheticHistory(158, sessions, 6, now.Add(-6*24*time.Hour))
		for _, p := range history {
			home.Prompt(p.Session, p.Dir, p.Text, p.At)
		}
		loader := state.NewLoader(home.Path, dead)
		_, err := loader.Load(now)
		require.NoError(t, err)

		t.Run("and a prompt is appended for a session whose wording changes no edge", func(t *testing.T) {
			before := loader.PairsScored()
			last := history[len(history)-1]
			home.Prompt(last.Session, last.Dir, "please zzqfirstword", now.Add(-time.Minute))
			snapshot, err := loader.Load(now.Add(time.Second))
			require.NoError(t, err)
			scored := loader.PairsScored() - before
			fresh, err := state.NewLoader(home.Path, dead).Load(now.Add(time.Second))
			require.NoError(t, err)

			t.Run("it should not score every pair again", func(t *testing.T) {
				assert.LessOrEqual(t, scored, sessions-1)
			})

			t.Run("it should show the same groups as a loader starting afresh", func(t *testing.T) {
				assert.Equal(t, fresh.Groups, snapshot.Groups)
			})
		})
	})
}
