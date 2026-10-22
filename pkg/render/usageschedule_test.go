package render

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/config"
)

// Item 79. Measured 2026-10-10: `claude -p "/usage"` reaches no model —
// zero tokens, zero cost, no API time — but it is a network fetch, so
// it runs only by hand unless usage_refresh names an interval.

func TestUsageDue(t *testing.T) {
	at := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	restore := timeNow
	timeNow = func() time.Time { return at }
	t.Cleanup(func() { timeNow = restore })
	watchedWindow(t, true)

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

	t.Run("when nobody is watching the window, and the reading is old", func(t *testing.T) {
		watchedWindow(t, false)
		due := game(30*time.Minute, 2*time.Hour, 0).usageDue()

		t.Run("it should say not to probe", func(t *testing.T) {
			assert.False(t, due)
		})
	})
}

// watchedWindow answers for the window manager: whether someone can see
// the city, for as long as the test runs.
func watchedWindow(t *testing.T, watched bool) {
	t.Helper()
	was := seen
	seen = func() (bool, bool, bool) { return watched, watched, !watched }
	t.Cleanup(func() { seen = was })
}

// Item 160. The slowest isolated /usage measured on 2026-10-21 took 45 s,
// and the budget for background helpers is 10% of the time.
const (
	measuredProbe = 45 * time.Second
	helperDuty    = 0.10
)

// probesOverAnHour steps the city's clock through an hour, a second at a
// time, letting the schedule probe whenever it is due, and counts the
// probes a fake started. Each probe takes measuredProbe, as one does.
func probesOverAnHour(t *testing.T, every time.Duration, watched bool) int {
	t.Helper()
	start := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	now := start
	restore := timeNow
	timeNow = func() time.Time { return now }
	t.Cleanup(func() { timeNow = restore })
	watchedWindow(t, watched)

	probes := 0
	g := &Game{usageEvery: every, home: t.TempDir()}
	g.usage = claude.Utilization{FetchedAt: start.Add(-24 * time.Hour)}
	g.probe = func(context.Context) { probes++ }
	for ; now.Before(start.Add(time.Hour)); now = now.Add(time.Second) {
		g.mu.Lock()
		busy := g.usageInFlight
		g.mu.Unlock()
		if !busy && g.usageDue() {
			g.refreshUsage()
			require.Eventually(t, func() bool {
				g.mu.Lock()
				defer g.mu.Unlock()
				return !g.usageInFlight
			}, 5*time.Second, time.Millisecond)
			now = now.Add(measuredProbe)
		}
	}
	return probes
}

func TestUsageProbesOverAnHour(t *testing.T) {
	for _, every := range []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour} {
		t.Run("when usage_refresh is "+every.String()+", and the window is watched for the hour", func(t *testing.T) {
			every, err := config.ParseUsageRefresh(every.String())
			require.NoError(t, err)
			probing := time.Duration(probesOverAnHour(t, every, true)) * measuredProbe

			t.Run("it should spend at most a tenth of the hour probing", func(t *testing.T) {
				assert.LessOrEqual(t, probing, time.Duration(helperDuty*float64(time.Hour)))
			})
		})
	}

	t.Run("when the window is open but unwatched for the hour", func(t *testing.T) {
		probes := probesOverAnHour(t, config.UsageRefreshFloor, false)

		t.Run("it should start no probe", func(t *testing.T) {
			assert.Zero(t, probes)
		})
	})
}
