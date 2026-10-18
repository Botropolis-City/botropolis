package state_test

import (
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// fakeStats is a statistics store: hefts before a refresh, and after.
type fakeStats struct {
	mu        sync.Mutex
	hefts     map[string]claude.Usage
	refreshed map[string]claude.Usage
	ingests   int
}

func (f *fakeStats) Ingest(string, time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ingests++
	if f.refreshed != nil {
		f.hefts = f.refreshed
	}
	return nil
}

func (f *fakeStats) Hefts() (map[string]claude.Usage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hefts, nil
}

// Item 110: hefts come from the statistics store, which outlives the
// transcripts, and the store is refreshed in the background.
func TestStoredHefts(t *testing.T) {
	const sid = "eeeeeeee-0000-0000-0000-000000000005"
	const gone = "ffffffff-0000-0000-0000-000000000006"
	bigHome := func(t *testing.T) *helpers.Home {
		t.Helper()
		start := now.Add(-7 * 24 * time.Hour)
		lines := []string{helpers.UserPrompt(sid, loaderCWD, start.Format(time.RFC3339Nano))}
		for i := range 1000 {
			at := start.Add(time.Duration(i) * 30 * time.Second).Format(time.RFC3339Nano)
			lines = append(lines, helpers.AssistantReply(sid, fmt.Sprintf("msg_%04d", i), at))
		}
		home := helpers.NewHome(t)
		home.Transcript(sid, loaderCWD, lines...)
		return home
	}
	stored := claude.Usage{Input: 1000, Output: 2000, CacheRead: 50_000}
	storedHeft := state.Heft(stored)
	goneHeft := state.Heft(claude.Usage{Output: 777})
	waitFor := func(t *testing.T, done <-chan struct{}) {
		t.Helper()
		select {
		case <-done:
		case <-time.After(30 * time.Second):
			require.FailNow(t, "the refresh never finished")
		}
	}
	load := func(t *testing.T, src *fakeStats, done func()) (*state.Loader, state.Snapshot) {
		t.Helper()
		loader := state.NewLoader(bigHome(t).Path, alive).WithStats(src, done)
		snapshot, err := loader.Load(now)
		require.NoError(t, err)
		return loader, snapshot
	}

	t.Run("when the store holds a stopped session's heft", func(t *testing.T) {
		_, snapshot := load(t, &fakeStats{hefts: map[string]claude.Usage{sid: stored}}, func() {})

		t.Run("it should use it", func(t *testing.T) {
			assert.Equal(t, storedHeft, sessionByID(t, snapshot, sid).Heft)
		})

		t.Run("it should not mark it partial", func(t *testing.T) {
			assert.False(t, sessionByID(t, snapshot, sid).HeftPartial)
		})
	})

	t.Run("when the store holds nothing for a session", func(t *testing.T) {
		_, snapshot := load(t, &fakeStats{hefts: map[string]claude.Usage{}}, func() {})

		t.Run("it should keep its usage, marked partial", func(t *testing.T) {
			assert.True(t, sessionByID(t, snapshot, sid).HeftPartial)
		})
	})

	t.Run("when the store held hefts at start-up, one of them for a session whose transcript is gone", func(t *testing.T) {
		_, snapshot := load(t, &fakeStats{hefts: map[string]claude.Usage{sid: stored, gone: {Output: 777}}}, func() {})

		t.Run("it should report the hefts read at once", func(t *testing.T) {
			assert.True(t, snapshot.HeftsRead)
		})

		t.Run("it should give the scale every stored heft, gone sessions included", func(t *testing.T) {
			got := slices.Clone(snapshot.ScaleHefts)
			slices.Sort(got)
			assert.Equal(t, []int64{goneHeft, storedHeft}, got)
		})
	})

	t.Run("when the store was empty at start-up", func(t *testing.T) {
		done := make(chan struct{}, 8)
		src := &fakeStats{hefts: map[string]claude.Usage{}, refreshed: map[string]claude.Usage{sid: stored}}
		loader, snapshot := load(t, src, func() { done <- struct{}{} })

		t.Run("it should not report them read", func(t *testing.T) {
			assert.False(t, snapshot.HeftsRead)
		})

		t.Run("and when the first refresh has passed", func(t *testing.T) {
			waitFor(t, done)
			after, err := loader.Load(now.Add(time.Second))
			require.NoError(t, err)

			t.Run("it should report them read", func(t *testing.T) {
				assert.True(t, after.HeftsRead)
			})

			t.Run("it should use what the refresh stored", func(t *testing.T) {
				assert.Equal(t, storedHeft, sessionByID(t, after, sid).Heft)
			})
		})
	})

	t.Run("when a refresh pass finishes", func(t *testing.T) {
		done := make(chan struct{}, 8)
		src := &fakeStats{hefts: map[string]claude.Usage{}}
		load(t, src, func() { done <- struct{}{} })
		waitFor(t, done)

		t.Run("it should have ingested", func(t *testing.T) {
			src.mu.Lock()
			defer src.mu.Unlock()
			assert.Equal(t, 1, src.ingests)
		})
	})
}
