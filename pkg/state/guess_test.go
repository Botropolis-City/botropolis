package state_test

import (
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
)

func TestGuessInputs(t *testing.T) {
	at := func(minutes int) time.Time { return now.Add(time.Duration(minutes) * time.Minute) }
	history := map[string][]claude.Prompt{
		"a": {{Text: "one", Dir: "/w/lib", At: at(-300)}, {Text: "two", Dir: "/w/lib", At: at(-30)}},
		"b": {{Text: "three", Dir: "/w/api", At: at(-120)}},
		"c": {{Text: "four", Dir: "/w/lib", At: at(-60)}},
	}

	t.Run("when the recently used directories are asked for", func(t *testing.T) {
		t.Run("it should list each once, the most recently prompted in first", func(t *testing.T) {
			assert.Equal(t, []string{"/w/lib", "/w/api"}, state.RecentDirs(history))
		})
	})

	t.Run("when a live session is weighed for New session", func(t *testing.T) {
		members := state.GuessMembers(history, []state.Session{{ID: "a", CWD: "/w/lib", State: state.Working}}, now)
		var a time.Time
		for _, m := range members {
			if m.ID == "a" {
				a = m.Ended
			}
		}

		t.Run("it should count it as left at its last prompt, so being just in it is a timing signal", func(t *testing.T) {
			assert.Equal(t, at(-30), a)
		})
	})
}
