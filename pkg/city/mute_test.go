package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 90: a session joined on wording offers to mute a word.
func TestMuteVerb(t *testing.T) {
	selected := func(t *testing.T, words []string) *city.Scene {
		t.Helper()
		a := session("a", cinders, state.Working)
		a.Group, a.GroupWords = "g1", words
		snap := snapshot(a, session("d", orchard, state.Working))
		snap.Groups = []state.Group{{ID: "g1", Name: "PROJ-1", Members: []string{"a"}}}
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(snap)
		require.True(t, s.Select("a"))
		return s
	}

	t.Run("when the selected session joined on wording", func(t *testing.T) {
		s := selected(t, []string{"kiln", "shelves"})

		t.Run("its verbs should include mute a word", func(t *testing.T) {
			assert.Contains(t, s.Actions(), city.ActionMuteWord)
		})

		t.Run("and mute a word is acted on", func(t *testing.T) {
			action, _ := s.Act(city.ActionMuteWord)

			t.Run("it should ask which of its words to mute", func(t *testing.T) {
				assert.Equal(t, city.Action{Kind: city.ActionMuteWord, SessionID: "a"}, action)
			})
		})
	})

	t.Run("when it joined on anything else", func(t *testing.T) {
		t.Run("its verbs should not include mute a word", func(t *testing.T) {
			assert.NotContains(t, selected(t, nil).Actions(), city.ActionMuteWord)
		})
	})
}
