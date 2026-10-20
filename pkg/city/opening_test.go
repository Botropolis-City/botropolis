package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// Item 169: every pass that "only takes note of the first snapshot"
// decided which snapshot was first by whether its map was still nil.
// Settings applied at start-up re-apply the scene's own empty snapshot
// before any real one arrives, which filled those maps, so the city
// opened by sailing in, building and celebrating everything already
// there.
func TestTheCityOpensOnWhatIsAlreadyThere(t *testing.T) {
	start := time.Date(2026, 10, 20, 13, 0, 0, 0, time.UTC)
	const openPR = 41
	working := func() state.Session {
		a := session("a", cinders, state.Working)
		a.SubagentsInFlight, a.SubagentDirs = 1, []string{cinders}
		a.PRs = []claude.PR{
			{Number: openPR, URL: "https://example.com/pr/41", State: claude.PROpen},
			{Number: 40, URL: "https://example.com/pr/40", State: claude.PRMerged},
		}
		return a
	}
	waiting := session("b", botropolis, state.Waiting)
	typing, _ := city.ParseTyping(city.TypingGuess)

	settings := []struct {
		name  string
		apply func(*city.Scene)
	}{
		{"nothing", func(*city.Scene) {}},
		{"a ledger", func(s *city.Scene) { s.SetLedger(&fakeLedger{}) }},
		{"a ladder", func(s *city.Scene) {
			s.SetLadder(state.Thresholds{Shuttered: time.Hour, Overgrown: 2 * time.Hour})
		}},
		{"a district typing", func(s *city.Scene) { s.SetTyping(typing) }},
		{"an unhide", func(s *city.Scene) { s.Unhide(cinders) }},
	}
	for _, setting := range settings {
		t.Run("when "+setting.name+" is applied before the first snapshot", func(t *testing.T) {
			opened := func(t *testing.T, clock *time.Time) *city.Scene {
				t.Helper()
				*clock = start
				s := city.NewScene(city.NewLayout())
				s.Resize(800, 600)
				s.SetClock(func() time.Time { return *clock }, time.UTC)
				setting.apply(s)
				s.SetSnapshot(snapshot(working(), waiting))
				return s
			}

			t.Run("and the first snapshot has live sessions", func(t *testing.T) {
				var clock time.Time
				s := opened(t, &clock)

				t.Run("it should launch no tugs", func(t *testing.T) {
					assert.Empty(t, s.Voyages())
				})
				t.Run("it should have the working session's building standing", func(t *testing.T) {
					assert.Equal(t, 1.0, s.Rising("a"))
				})
				t.Run("it should find its drone already out, not taking off", func(t *testing.T) {
					flights := s.Flights("a")
					require.Len(t, flights, 1)
					assert.NotEqual(t, city.TakingOff, flights[0].Stage)
				})
				t.Run("it should show its open PR's flag already up", func(t *testing.T) {
					assert.Equal(t, 1.0, s.Hoist("a", openPR))
				})
				t.Run("it should not celebrate a merge that happened before it opened", func(t *testing.T) {
					assert.Equal(t, -1.0, s.Celebration("a"))
				})
			})

			t.Run("and a later snapshot adds a session", func(t *testing.T) {
				var clock time.Time
				s := opened(t, &clock)
				clock = clock.Add(time.Second)
				s.SetSnapshot(snapshot(working(), waiting, session("c", cinders, state.Working)))

				t.Run("it should sail that one in", func(t *testing.T) {
					var ids []string
					for _, v := range s.Voyages() {
						ids = append(ids, v.SessionID)
					}
					assert.Equal(t, []string{"c"}, ids)
				})
			})
		})
	}
}
