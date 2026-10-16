package state_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/state"
)

func TestOrder(t *testing.T) {
	t.Run("when states are ranked", func(t *testing.T) {
		ranks := []struct {
			state state.State
			rank  int
		}{
			{state.NeedsYou, 0},
			{state.Working, 1},
			{state.Waiting, 2},
			{state.Unattended, 3},
			{state.Resting, 4},
		}
		for _, tc := range ranks {
			t.Run(fmt.Sprintf("it should rank %s at %d", tc.state, tc.rank), func(t *testing.T) {
				assert.Equal(t, tc.rank, state.Rank(tc.state))
			})
		}

		t.Run("it should rank an unknown state after every known one", func(t *testing.T) {
			assert.Equal(t, len(state.Order), state.Rank(state.State("gone")))
		})
	})

	t.Run("when sessions are tallied", func(t *testing.T) {
		counts := state.Tally([]state.Session{
			{State: state.Resting}, {State: state.NeedsYou}, {State: state.Resting}, {State: state.Working},
		})

		t.Run("it should count each state", func(t *testing.T) {
			assert.Equal(t, map[state.State]int{state.NeedsYou: 1, state.Working: 1, state.Resting: 2}, counts)
		})
	})

	t.Run("when counts are walked in order", func(t *testing.T) {
		counts := state.Nonzero(map[state.State]int{state.Resting: 4, state.NeedsYou: 2, state.Working: 0, state.Unattended: 1})

		t.Run("it should list the non-zero states by urgency", func(t *testing.T) {
			assert.Equal(t, []state.Count{{state.NeedsYou, 2}, {state.Unattended, 1}, {state.Resting, 4}}, counts)
		})
	})

	t.Run("when an empty session is tallied", func(t *testing.T) {
		counts := state.Tally([]state.Session{{State: state.Empty}, {State: state.Working}})

		t.Run("it should never be counted", func(t *testing.T) {
			assert.Equal(t, []state.Count{{state.Working, 1}}, state.Nonzero(counts))
		})

		t.Run("it should rank after parked", func(t *testing.T) {
			assert.Greater(t, state.Rank(state.Empty), state.Rank(state.Resting))
		})

		t.Run("it should not be live", func(t *testing.T) {
			assert.False(t, state.Live(state.Empty))
		})
	})

	t.Run("when only live counts are walked", func(t *testing.T) {
		counts := state.Nonzero(map[state.State]int{state.Resting: 4, state.Working: 3}, state.Live)

		t.Run("it should leave parked out", func(t *testing.T) {
			assert.Equal(t, []state.Count{{state.Working, 3}}, counts)
		})
	})
}
