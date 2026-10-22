package daemon_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/daemon"
	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/state"
)

// scanningLoader is a loader partway through a long read of the history,
// whose progress the test moves.
type scanningLoader struct {
	loads atomic.Int64
	mu    sync.Mutex
	scan  *state.ScanProgress
}

func (s *scanningLoader) Load(now time.Time) (state.Snapshot, error) {
	s.loads.Add(1)
	return state.Snapshot{At: now, Scan: s.Progress()}, nil
}

func (s *scanningLoader) WatchDirs() []string { return nil }

func (s *scanningLoader) Progress() *state.ScanProgress {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.scan
}

func (s *scanningLoader) set(scan *state.ScanProgress) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scan = scan
}

// Item 159: the city shows a first launch's progress, which moves
// between loads.
func TestDaemonSendsProgress(t *testing.T) {
	const every = 10 * time.Millisecond
	next := func(t *testing.T, updates <-chan proto.Update) (proto.Update, bool) {
		t.Helper()
		select {
		case u := <-updates:
			return u, true
		case <-time.After(20 * every):
			return proto.Update{}, false
		}
	}

	t.Run("when a scan is in progress and moves on after the rescan", func(t *testing.T) {
		loader := &scanningLoader{scan: &state.ScanProgress{Done: 0, Known: 64 << 20}}
		d := daemon.NewWith(loader, func() time.Time { return now })
		d.SetProgressEvery(every)
		updates, stop := d.Subscribe()
		t.Cleanup(stop)
		require.NoError(t, d.Rescan())
		_, ok := next(t, updates)
		require.True(t, ok)
		loads := loader.loads.Load()
		loader.set(&state.ScanProgress{Done: 16 << 20, Known: 64 << 20})
		moved, ok := next(t, updates)
		require.True(t, ok)

		t.Run("it should send subscribers the new progress", func(t *testing.T) {
			assert.Equal(t, &state.ScanProgress{Done: 16 << 20, Known: 64 << 20}, moved.Snapshot.Scan)
		})

		t.Run("it should not load again to send it", func(t *testing.T) {
			assert.Equal(t, loads, loader.loads.Load())
		})

		t.Run("and the scan finishes", func(t *testing.T) {
			loader.set(nil)
			finished, ok := next(t, updates)
			require.True(t, ok)

			t.Run("it should send that there is no scan any more", func(t *testing.T) {
				assert.Nil(t, finished.Snapshot.Scan)
			})

			t.Run("it should stop sending", func(t *testing.T) {
				_, more := next(t, updates)
				assert.False(t, more)
			})
		})
	})

	t.Run("when the progress has not moved", func(t *testing.T) {
		loader := &scanningLoader{scan: &state.ScanProgress{Done: 0, Known: 64 << 20}}
		d := daemon.NewWith(loader, func() time.Time { return now })
		d.SetProgressEvery(every)
		updates, stop := d.Subscribe()
		t.Cleanup(stop)
		require.NoError(t, d.Rescan())
		_, ok := next(t, updates)
		require.True(t, ok)
		_, more := next(t, updates)

		t.Run("it should send nothing new", func(t *testing.T) {
			assert.False(t, more)
		})
	})
}
