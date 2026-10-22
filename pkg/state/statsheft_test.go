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

// fakeStats is a statistics store: hefts at start-up, and the sessions
// the next ingest moves, with their counters after it.
type fakeStats struct {
	mu      sync.Mutex
	hefts   map[string]claude.Usage
	next    *move
	moved   *move
	ingests int
}

type move struct {
	hefts map[string]claude.Usage
	ids   []string
}

func (f *fakeStats) Ingest(string, time.Time, func()) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ingests++
	f.moved, f.next = f.next, nil
	return nil
}

func (f *fakeStats) MovedHefts() (map[string]claude.Usage, []string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m := f.moved
	f.moved = nil
	if m == nil {
		return map[string]claude.Usage{}, nil, nil
	}
	return m.hefts, m.ids, nil
}

func (f *fakeStats) ingested() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ingests
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
		src := &fakeStats{hefts: map[string]claude.Usage{}, next: &move{hefts: map[string]claude.Usage{sid: stored}, ids: []string{sid}}}
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

	// Item 196: a pass reads again only the sessions the store says
	// moved.
	t.Run("when the store held hefts at start-up", func(t *testing.T) {
		grown := claude.Usage{Input: 1000, Output: 9000, CacheRead: 50_000}
		startUp := func() map[string]claude.Usage { return map[string]claude.Usage{sid: stored, gone: {Output: 777}} }
		scaleAfter := func(t *testing.T, loader *state.Loader) []int64 {
			t.Helper()
			after, err := loader.Load(now.Add(time.Second))
			require.NoError(t, err)
			got := slices.Clone(after.ScaleHefts)
			slices.Sort(got)
			return got
		}

		t.Run("and a pass finds a session moved", func(t *testing.T) {
			done := make(chan struct{}, 8)
			src := &fakeStats{hefts: startUp(), next: &move{hefts: map[string]claude.Usage{sid: grown}, ids: []string{sid}}}
			loader, _ := load(t, src, func() { done <- struct{}{} })
			waitFor(t, done)

			t.Run("it should take that session's new heft and keep the others", func(t *testing.T) {
				assert.Equal(t, []int64{goneHeft, state.Heft(grown)}, scaleAfter(t, loader))
			})
		})

		t.Run("and a pass finds a moved session with no counters", func(t *testing.T) {
			done := make(chan struct{}, 8)
			src := &fakeStats{hefts: startUp(), next: &move{hefts: map[string]claude.Usage{}, ids: []string{gone}}}
			loader, _ := load(t, src, func() { done <- struct{}{} })
			waitFor(t, done)

			t.Run("it should drop its heft", func(t *testing.T) {
				assert.Equal(t, []int64{storedHeft}, scaleAfter(t, loader))
			})
		})

		t.Run("and a pass finds nothing moved", func(t *testing.T) {
			done := make(chan struct{}, 8)
			src := &fakeStats{hefts: startUp()}
			loader, _ := load(t, src, func() { done <- struct{}{} })
			require.Eventually(t, func() bool {
				_, err := loader.Load(now.Add(time.Second))
				return err == nil && src.ingested() >= 2
			}, 30*time.Second, 10*time.Millisecond)

			t.Run("it should not call done", func(t *testing.T) {
				assert.Empty(t, done)
			})
		})
	})
}
