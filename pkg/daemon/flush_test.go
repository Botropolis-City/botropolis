package daemon_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/daemon"
	"botropolis.city/botropolis/pkg/state"
)

type flushingLoader struct{ flushes int }

func (f *flushingLoader) Load(now time.Time) (state.Snapshot, error) {
	return state.Snapshot{At: now}, nil
}

func (f *flushingLoader) WatchDirs() []string { return nil }

func (f *flushingLoader) Flush() error {
	f.flushes++
	return nil
}

// Item 205: the ledger saves a change to last activity alone at most once
// a minute, so the daemon saves what it holds when it stops.
func TestDaemonFlush(t *testing.T) {
	t.Run("when the daemon is flushed", func(t *testing.T) {
		loader := &flushingLoader{}
		d := daemon.NewWith(loader, func() time.Time { return now })
		require.NoError(t, d.Flush())

		t.Run("it should flush its loader", func(t *testing.T) {
			assert.Equal(t, 1, loader.flushes)
		})
	})

	t.Run("when its loader keeps nothing to flush", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))

		t.Run("it should do nothing, without an error", func(t *testing.T) {
			assert.NoError(t, d.Flush())
		})
	})
}
