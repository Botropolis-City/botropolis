package state_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// yieldingStats is a statistics store whose ingest waits to be told to
// go on, then yields once.
type yieldingStats struct {
	ingesting chan struct{}
	proceed   chan struct{}
	yielded   chan struct{}
}

func (y *yieldingStats) Ingest(_ string, _ time.Time, yield func()) error {
	close(y.ingesting)
	<-y.proceed
	yield()
	close(y.yielded)
	return nil
}

func (y *yieldingStats) Hefts() (map[string]claude.Usage, error) {
	return map[string]claude.Usage{}, nil
}

// Item 138: the backfill gives way while a load runs.
func TestStoredHeftsYield(t *testing.T) {
	t.Run("when a load is running and the pass yields", func(t *testing.T) {
		const sid = "eeeeeeee-0000-0000-0000-000000000007"
		home := helpers.NewHome(t).Session(7007, sid, loaderCWD, "interactive", "idle")
		home.Transcript(sid, loaderCWD, helpers.UserPrompt(sid, loaderCWD, now.Format(time.RFC3339Nano)))
		var block atomic.Bool
		inLoad, release := make(chan struct{}), make(chan struct{})
		probes := state.Probes{
			Alive: func(int) bool {
				if block.CompareAndSwap(true, false) {
					close(inLoad)
					<-release
				}
				return true
			},
			Attached: func(string) bool { return false },
		}
		src := &yieldingStats{ingesting: make(chan struct{}), proceed: make(chan struct{}), yielded: make(chan struct{})}
		loader := state.NewLoader(home.Path, probes).WithStats(src, nil)
		_, err := loader.Load(now)
		require.NoError(t, err)
		<-src.ingesting
		block.Store(true)
		loaded := make(chan struct{})
		go func() {
			defer close(loaded)
			_, _ = loader.Load(now)
		}()
		<-inLoad
		close(src.proceed)
		t.Cleanup(func() {
			close(release)
			<-loaded
			<-src.yielded
		})
		wentOn := false
		select {
		case <-src.yielded:
			wentOn = true
		case <-time.After(200 * time.Millisecond):
		}

		t.Run("it should not go on until the load returns", func(t *testing.T) {
			assert.False(t, wentOn)
		})
	})
}
