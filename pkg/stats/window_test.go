package stats_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/stats"
)

var windowEnd = time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

func (f *fixture) window(hours int) stats.Window {
	f.t.Helper()
	w, err := f.store.Window(windowEnd.Add(-time.Duration(hours)*time.Hour), windowEnd, hours, city.ProjectRoot, nil)
	require.NoError(f.t, err)
	return w
}

func keys(slices []stats.Slice) []string {
	out := []string{}
	for _, s := range slices {
		out = append(out, s.Key)
	}
	return out
}

func outputs(slices []stats.Slice) map[string]int64 {
	out := map[string]int64{}
	for _, s := range slices {
		out[s.Key] = s.Counters.Output
	}
	return out
}

func TestWindow(t *testing.T) {
	t.Run("when messages fall inside and outside the window", func(t *testing.T) {
		f := newFixture(t)
		f.seed(
			msg("msg_1", "2026-10-18T10:30:00.000Z", lib, usage{Output: 1}),
			msg("msg_2", "2026-10-18T09:30:00.000Z", lib, usage{Output: 2}),
			msg("msg_3", "2026-10-18T12:30:00.000Z", lib, usage{Output: 4}),
		)

		t.Run("it should total only those inside", func(t *testing.T) {
			assert.Equal(t, int64(1), f.window(2).Counters.Output)
		})
	})

	t.Run("when a session changed model between messages", func(t *testing.T) {
		f := newFixture(t)
		f.seed(
			modelMsg("msg_1", "2026-10-18T10:00:00.000Z", "claude-haiku-4-5", usage{Output: 1}),
			modelMsg("msg_2", "2026-10-18T11:00:00.000Z", "claude-opus-5-5", usage{Output: 2}),
		)

		t.Run("it should credit each message to its own model", func(t *testing.T) {
			assert.Equal(t, map[string]int64{"claude-haiku-4-5": 1, "claude-opus-5-5": 2}, outputs(f.window(24).ByModel))
		})

		t.Run("it should order the models by cost, dearest first", func(t *testing.T) {
			assert.Equal(t, []string{"claude-opus-5-5", "claude-haiku-4-5"}, keys(f.window(24).ByModel))
		})
	})

	t.Run("when a message is in a worktree of a project", func(t *testing.T) {
		f := newFixture(t)
		f.seed(
			msg("msg_1", "2026-10-18T10:00:00.000Z", lib, usage{Output: 1}),
			msg("msg_2", "2026-10-18T11:00:00.000Z", libWorktree, usage{Output: 2}),
		)

		t.Run("it should count it under the project", func(t *testing.T) {
			assert.Equal(t, map[string]int64{lib: 3}, outputs(f.window(24).ByProject))
		})
	})

	t.Run("when a fork holds its parent's message", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA, msg("msg_1", "2026-10-18T10:00:00.000Z", lib, usage{Output: 5}))
		f.transcript(sessionB, reply{session: sessionB, id: "msg_1", at: "2026-10-18T10:00:00.000Z", cwd: lib, usage: usage{Output: 5}}.line())
		f.ingest()
		w := f.window(24)

		t.Run("it should count it in each session that holds it", func(t *testing.T) {
			assert.Equal(t, map[string]int64{sessionA: 5, sessionB: 5}, outputs(w.BySession))
		})

		t.Run("it should count it once in the total", func(t *testing.T) {
			assert.Equal(t, int64(5), w.Counters.Output)
		})
	})

	t.Run("when the window is three hours", func(t *testing.T) {
		f := newFixture(t)
		f.seed(
			modelMsg("msg_1", "2026-10-18T09:30:00.000Z", "claude-opus-5-5", usage{Output: million}),
			modelMsg("msg_2", "2026-10-18T11:59:00.000Z", "claude-opus-5-5", usage{Output: 2 * million}),
		)
		w := f.window(3)

		t.Run("it should give one cost an hour", func(t *testing.T) {
			assert.Len(t, w.Hourly, 3)
		})

		t.Run("it should put the oldest hour first", func(t *testing.T) {
			assert.InDelta(t, 20, w.Hourly[0].Low, 1e-9)
		})

		t.Run("it should put the latest hour last", func(t *testing.T) {
			assert.InDelta(t, 40, w.Hourly[2].Low, 1e-9)
		})
	})
}
