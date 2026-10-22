package daemon_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/daemon"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// loadClock is a loader that notes when each load starts.
type loadClock struct {
	*state.Loader
	mu    sync.Mutex
	loads []time.Time
}

func (c *loadClock) Load(now time.Time) (state.Snapshot, error) {
	c.mu.Lock()
	c.loads = append(c.loads, time.Now())
	c.mu.Unlock()
	return c.Loader.Load(now)
}

// since is the starts of the loads at or after from.
func (c *loadClock) since(from time.Time) []time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	var at []time.Time
	for _, l := range c.loads {
		if !l.Before(from) {
			at = append(at, l)
		}
	}
	return at
}

// Item 196: each write was its own rescan, so a session writing every
// 200 ms cost five whole loads a second. Rescans now come at most once a
// second, and the first after a quiet spell still within the debounce.
func TestWatchPace(t *testing.T) {
	const (
		every   = 200 * time.Millisecond
		burst   = 3 * time.Second
		paced   = time.Second
		leading = 300 * time.Millisecond
		// slack is what a busy machine may add to a timer.
		slack = 200 * time.Millisecond
	)
	start := func(t *testing.T) (*helpers.Home, *loadClock) {
		t.Helper()
		home := idleHome(t, "interactive")
		c := &loadClock{Loader: state.NewLoader(home.Path, probes)}
		d := daemon.NewWith(c, func() time.Time { return now })
		require.NoError(t, d.Rescan())
		watch(t, d)
		time.Sleep(paced + slack)
		return home, c
	}
	write := func(home *helpers.Home, i int) {
		home.AppendTranscript(sid, cwd, helpers.AssistantReply(sid, fmt.Sprintf("msg_pace_%d", i), stamp(time.Duration(20+i)*time.Second)))
	}

	t.Run("when a session writes every 200 ms for 3 s", func(t *testing.T) {
		home, c := start(t)
		from := time.Now()
		for i := 0; time.Since(from) < burst; i++ {
			write(home, i)
			time.Sleep(every)
		}
		during := len(c.since(from))

		t.Run("it should rescan at most once a second", func(t *testing.T) {
			assert.LessOrEqual(t, during, int(burst/paced)+1)
		})
	})

	t.Run("when a burst of writes stops", func(t *testing.T) {
		home, c := start(t)
		var last time.Time
		for i := range 4 {
			write(home, i)
			last = time.Now()
			time.Sleep(every)
		}
		time.Sleep(paced + slack)
		after := c.since(last)

		t.Run("it should rescan once more within a second of the last", func(t *testing.T) {
			require.NotEmpty(t, after)
			assert.LessOrEqual(t, after[0].Sub(last), paced+slack)
		})
	})

	t.Run("when a session writes after a quiet spell", func(t *testing.T) {
		home, c := start(t)
		at := time.Now()
		write(home, 0)
		time.Sleep(paced)
		after := c.since(at)

		t.Run("it should rescan within the debounce", func(t *testing.T) {
			require.NotEmpty(t, after)
			assert.LessOrEqual(t, after[0].Sub(at), leading)
		})
	})
}
