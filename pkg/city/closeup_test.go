package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 86, slice 4: what closing up would stop.
func TestCloseUpList(t *testing.T) {
	older := session("older", cinders, state.NeedsYou)
	older.LastActivity = now.Add(-time.Hour)
	newer := session("newer", botropolis, state.Working)
	newer.LastActivity = now.Add(-time.Minute)
	crate := session("crate", cinders, state.Working)
	crate.Packed = true
	s := scene(t, older, newer, session("rest", cinders, state.Resting), session("empty", cinders, state.Empty), crate)
	ids := func(sessions []state.Session) []string {
		var out []string
		for _, x := range sessions {
			out = append(out, x.ID)
		}
		return out
	}

	t.Run("when the desk is about to be closed up", func(t *testing.T) {
		t.Run("it should list every live session and no resting, empty or packed one", func(t *testing.T) {
			assert.ElementsMatch(t, []string{"older", "newer"}, ids(s.OpenSessions()))
		})

		t.Run("it should list them most recently active first", func(t *testing.T) {
			assert.Equal(t, []string{"newer", "older"}, ids(s.OpenSessions()))
		})
	})

	t.Run("when the actor is asked to close up", func(t *testing.T) {
		action := city.Action{Kind: city.ActionCloseUp, Open: []string{"newer"}, Pinned: []string{"newer"}}

		t.Run("its description should say what it does", func(t *testing.T) {
			assert.NotEmpty(t, action.Kind.Describe())
		})
	})
}
