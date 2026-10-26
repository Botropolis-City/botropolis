package city_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/stats"
)

const (
	keptDir = "/home/someone/projects/lantern"
	goneDir = "/home/someone/projects/old-kiln"
)

// Item 274: a session whose transcript Claude Code deleted leaves the
// map unless the setting keeps it; its statistics stay.
func TestGoneSessions(t *testing.T) {
	kept := session("kept", keptDir, state.Resting)
	gone := session("gone", goneDir, state.Resting)
	gone.ConversationGone = true

	scene := func(shown city.Shown) *city.Scene {
		s := city.NewScene(city.NewLayout())
		s.SetShown(shown)
		s.SetSnapshot(snapshot(kept, gone))
		return s
	}
	has := func(s *city.Scene, id string) bool {
		for _, b := range s.City().Buildings() {
			if b.Session.ID == id {
				return true
			}
		}
		return false
	}
	hasDistrict := func(s *city.Scene, root string) bool {
		for _, d := range s.City().Districts {
			if d.Root == root {
				return true
			}
		}
		return false
	}

	t.Run("when a snapshot holds a session whose transcript is gone", func(t *testing.T) {
		t.Run("and deleted transcripts are hidden, as by default", func(t *testing.T) {
			s := city.NewScene(city.NewLayout())
			s.SetSnapshot(snapshot(kept, gone))

			t.Run("it should draw no building for it", func(t *testing.T) {
				assert.False(t, has(s, "gone"))
			})

			t.Run("it should draw no district made only of such sessions", func(t *testing.T) {
				assert.False(t, hasDistrict(s, goneDir))
			})
		})

		t.Run("and deleted transcripts are shown", func(t *testing.T) {
			s := scene(city.Shown{Gone: true})

			t.Run("it should draw its building", func(t *testing.T) {
				assert.True(t, has(s, "gone"))
			})
		})

		t.Run("and the setting is turned on after the snapshot arrived", func(t *testing.T) {
			s := scene(city.Shown{})
			s.SetShown(city.Shown{Gone: true})

			t.Run("it should draw its building without a new snapshot", func(t *testing.T) {
				assert.True(t, has(s, "gone"))
			})
		})

		t.Run("and statistics hold its spend", func(t *testing.T) {
			spendWith := func(shown city.Shown) float64 {
				s := city.NewScene(city.NewLayout())
				s.SetShown(shown)
				s.SetLedger(&fakeLedger{
					window:   stats.Window{Cost: cost(7.5), BySession: []stats.Slice{{Key: "gone", Cost: cost(5)}, {Key: "kept", Cost: cost(2.5)}}},
					sessions: map[string]stats.Cost{"gone": cost(5), "kept": cost(2.5)},
				})
				s.SetSnapshot(snapshot(kept, gone))
				return s.City().Plant.Day.Cost.High
			}

			t.Run("it should count the same day's spend hidden as shown", func(t *testing.T) {
				assert.Equal(t, spendWith(city.Shown{Gone: true}), spendWith(city.Shown{}))
			})
		})
	})

	t.Run("when a session's transcript exists", func(t *testing.T) {
		t.Run("and deleted transcripts are hidden", func(t *testing.T) {
			s := scene(city.Shown{})

			t.Run("it should keep its building", func(t *testing.T) {
				assert.True(t, has(s, "kept"))
			})
		})
	})
}
