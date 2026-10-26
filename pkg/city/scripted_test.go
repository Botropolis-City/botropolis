package city_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 273: a session a program started is off the map unless
// scripted_runs shows it, and its tokens count either way.
func TestScriptedRuns(t *testing.T) {
	run := spender("run", "/src/pipeline", "claude-opus-5", map[int]int64{1: 700})
	run.Entrypoint = "sdk-cli"
	person := spender("person", "/src/kiln", "claude-opus-5", map[int]int64{1: 300})
	person.Entrypoint = "cli"

	building := func(s *city.Scene, id string) bool {
		for _, b := range s.City().Buildings() {
			if b.Session.ID == id {
				return true
			}
		}
		return false
	}
	district := func(s *city.Scene, root string) bool {
		for _, d := range s.City().Districts {
			if d.Root == root {
				return true
			}
		}
		return false
	}

	t.Run("when a scripted session is in the snapshot", func(t *testing.T) {
		t.Run("and the setting is left hidden", func(t *testing.T) {
			s := scene(t, run, person)

			t.Run("it should make no building for it", func(t *testing.T) {
				assert.False(t, building(s, "run"))
			})

			t.Run("and it is its district's only session, it should make no district", func(t *testing.T) {
				assert.False(t, district(s, "/src/pipeline"))
			})

			t.Run("and there is no statistics store, it should still count its tokens in the plant's day", func(t *testing.T) {
				assert.Equal(t, int64(1000), s.City().Plant.Day.Counters.Output)
			})

			t.Run("it should still make the interactive session's building", func(t *testing.T) {
				assert.True(t, building(s, "person"))
			})
		})

		t.Run("and the setting shows them", func(t *testing.T) {
			s := scene(t)
			s.SetShown(city.Shown{Scripted: true})
			s.SetSnapshot(snapshot(run, person))

			t.Run("it should make a building for it", func(t *testing.T) {
				assert.True(t, building(s, "run"))
			})

			t.Run("it should count its tokens once", func(t *testing.T) {
				assert.Equal(t, int64(1000), s.City().Plant.Day.Counters.Output)
			})
		})

		t.Run("and the setting is turned on while the map is open", func(t *testing.T) {
			s := scene(t, run, person)
			s.SetShown(city.Shown{Scripted: true})

			t.Run("it should make a building for it at once", func(t *testing.T) {
				assert.True(t, building(s, "run"))
			})
		})
	})

	t.Run("when a session names no entrypoint", func(t *testing.T) {
		s := scene(t, session("unknown", "/src/kiln", state.Working))

		t.Run("it should make a building for it", func(t *testing.T) {
			assert.True(t, building(s, "unknown"))
		})
	})
}
