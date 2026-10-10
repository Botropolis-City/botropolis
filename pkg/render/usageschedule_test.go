package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/claude"
)

// Item 79. Measured 2026-10-10: `claude -p "/usage"` reaches no model —
// zero tokens, zero cost, no API time — but it is a network fetch, so
// it runs only by hand unless usage_refresh names an interval.

func TestUsageDue(t *testing.T) {
	at := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	restore := timeNow
	timeNow = func() time.Time { return at }
	t.Cleanup(func() { timeNow = restore })

	game := func(every time.Duration, fetched, asked time.Duration) *Game {
		g := &Game{usageEvery: every}
		g.usage = claude.Utilization{FetchedAt: at.Add(-fetched)}
		if asked > 0 {
			g.usageAskedAt = at.Add(-asked)
		}
		return g
	}

	for _, c := range []struct {
		name string
		g    func() *Game
		due  bool
	}{
		{"when refreshing is left to the key, and the reading is old", func() *Game { return game(0, 24*time.Hour, 0) }, false},
		{"when the reading is older than the interval", func() *Game { return game(30*time.Minute, 31*time.Minute, 0) }, true},
		{"when the reading is younger than the interval", func() *Game { return game(30*time.Minute, 10*time.Minute, 0) }, false},
		{"when the reading is old, and a probe went out recently", func() *Game { return game(30*time.Minute, 2*time.Hour, time.Minute) }, false},
		{"when there is no reading at all", func() *Game { return &Game{usageEvery: 30 * time.Minute} }, true},
		{"when the city is recording, and the reading is old", func() *Game {
			g := game(30*time.Minute, 2*time.Hour, 0)
			g.record = "frames"
			return g
		}, false},
		{"when the city is taking a still, and the reading is old", func() *Game {
			g := game(30*time.Minute, 2*time.Hour, 0)
			g.screenshot = "still.png"
			return g
		}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Run("it should say whether to probe", func(t *testing.T) {
				assert.Equal(t, c.due, c.g().usageDue())
			})
		})
	}
}
