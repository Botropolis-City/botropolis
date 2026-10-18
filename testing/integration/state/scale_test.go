package state_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/harness"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/stats"
	"botropolis.city/botropolis/testing/helpers"
)

const cwd = "/w/lib"

// daemon is one run of the daemon's loading, on the statistics store at
// path: it opens the store, loads once, and waits for the first refresh.
type daemon struct {
	store *stats.Store
	multi *harness.Multi
	done  chan struct{}
}

func start(t *testing.T, home, path string) *daemon {
	t.Helper()
	store, err := stats.Open(path)
	require.NoError(t, err)
	d := &daemon{store: store, done: make(chan struct{}, 16)}
	probes := state.Probes{Alive: func(int) bool { return false }, Attached: func(string) bool { return false }}
	loader := state.NewLoader(home, probes).WithStats(store, func() { d.done <- struct{}{} })
	d.multi = harness.NewMulti(harness.NewClaude(loader))
	return d
}

func (d *daemon) load(t *testing.T, at time.Time) state.Snapshot {
	t.Helper()
	snapshot, err := d.multi.Load(at)
	require.NoError(t, err)
	return snapshot
}

func (d *daemon) settle(t *testing.T) {
	t.Helper()
	select {
	case <-d.done:
	case <-time.After(30 * time.Second):
		require.FailNow(t, "the refresh never finished")
	}
}

func (d *daemon) stop(t *testing.T) {
	t.Helper()
	require.NoError(t, d.store.Close())
}

func storeys(snapshot state.Snapshot, ids []string) []int {
	by := map[string]int{}
	for _, s := range snapshot.Sessions {
		by[s.ID] = s.Storeys
	}
	out := make([]int, len(ids))
	for i, id := range ids {
		out[i] = by[id]
	}
	return out
}

// Item 110: the scale is cut from the statistics store, so a transcript
// Claude Code deletes does not move it, and a restart starts on it.
func TestScaleOutlivesTranscripts(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	home := helpers.NewHome(t)
	var ids, paths []string
	for i := range 24 {
		id := fmt.Sprintf("%08x-0000-0000-0000-%012x", i+1, i+1)
		start := now.Add(-time.Duration(48-i) * time.Hour)
		lines := []string{helpers.UserPrompt(id, cwd, start.Format(time.RFC3339Nano))}
		for r := range i + 1 {
			at := start.Add(time.Duration(r) * time.Minute).Format(time.RFC3339Nano)
			lines = append(lines, helpers.AssistantReply(id, fmt.Sprintf("msg_%02d_%02d", i, r), at))
		}
		ids = append(ids, id)
		paths = append(paths, home.Transcript(id, cwd, lines...))
	}
	path := filepath.Join(t.TempDir(), "stats.db")

	first := start(t, home.Path, path)
	first.load(t, now)
	first.settle(t)
	before := first.load(t, now.Add(time.Second))
	first.stop(t)

	kept := ids[:20]
	for _, p := range paths[20:] {
		require.NoError(t, os.Remove(p))
	}

	t.Run("when the daemon restarts after stored transcripts were deleted", func(t *testing.T) {
		again := start(t, home.Path, path)
		t.Cleanup(func() { again.stop(t) })
		after := again.load(t, now.Add(time.Minute))

		t.Run("it should cut the same scale as before", func(t *testing.T) {
			assert.Equal(t, storeys(before, kept), storeys(after, kept))
		})

		t.Run("it should start on the user's own scale, not the starter lines", func(t *testing.T) {
			assert.True(t, after.HeftsRead)
		})
	})
}
