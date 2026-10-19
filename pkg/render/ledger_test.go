package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/stats"
)

// Item 116: the city reads its usage through a ledger while statistics
// are on.
func TestStatsLedger(t *testing.T) {
	now := time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)
	newGame := func(t *testing.T) *Game {
		t.Helper()
		t.Setenv("XDG_STATE_HOME", t.TempDir())
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(state.Snapshot{At: now})
		g := &Game{scene: s}
		t.Cleanup(func() { g.setStatsOn(false) })
		return g
	}
	reason := func(g *Game) string { return g.scene.City().Plant.Day.Reason }

	t.Run("when statistics are turned on", func(t *testing.T) {
		g := newGame(t)
		g.setStatsOn(true)

		t.Run("it should give the game a ledger", func(t *testing.T) {
			assert.NotNil(t, g.ledger)
		})

		t.Run("it should have the city read through it", func(t *testing.T) {
			assert.Equal(t, "there are no statistics yet", reason(g))
		})

		t.Run("and turned off again", func(t *testing.T) {
			g.setStatsOn(false)

			t.Run("it should drop the ledger", func(t *testing.T) {
				assert.Nil(t, g.ledger)
			})

			t.Run("it should have the city say statistics are off", func(t *testing.T) {
				assert.Equal(t, stats.ReasonOff, reason(g))
			})
		})
	})

	t.Run("when the on prompt is answered", func(t *testing.T) {
		g := newGame(t)
		(&statsPrompt{kind: stats.OnPrompt}).answer(g, true)

		t.Run("it should give the game a ledger", func(t *testing.T) {
			assert.NotNil(t, g.ledger)
		})
	})
}
