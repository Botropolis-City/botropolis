package daemon_test

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/daemon"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 205: on a home of 2,000 sessions a rescan costs more than 1% of a
// core at one a second, so the watcher waits after each rescan until it
// has cost no more than that share: at least a second, as item 196 set,
// and at most 5 s, so nothing read from a transcript is later than that.
func TestPaceAfter(t *testing.T) {
	cases := []struct {
		cost, want time.Duration
	}{
		{cost: 2 * time.Millisecond, want: time.Second},
		{cost: 10 * time.Millisecond, want: time.Second},
		{cost: 15 * time.Millisecond, want: 1500 * time.Millisecond},
		{cost: 30 * time.Millisecond, want: 3 * time.Second},
		{cost: 50 * time.Millisecond, want: 5 * time.Second},
		{cost: 200 * time.Millisecond, want: 5 * time.Second},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("when a rescan costs %v, it should wait %v", c.cost, c.want), func(t *testing.T) {
			assert.Equal(t, c.want, daemon.PaceAfter(c.cost))
		})
	}
}

func TestWatchPacesByCost(t *testing.T) {
	const (
		cost  = 30 * time.Millisecond
		every = 200 * time.Millisecond
		burst = 7 * time.Second
		slack = 300 * time.Millisecond
	)
	t.Run("when every rescan costs 30 ms and a session writes every 200 ms for 7 s", func(t *testing.T) {
		home := idleHome(t, "interactive")
		c := &loadClock{Loader: state.NewLoader(home.Path, probes)}
		d := daemon.NewWith(c, func() time.Time { return now })
		var calls atomic.Int64
		daemon.SetThreadCPU(d, func() time.Duration { return time.Duration(calls.Add(1)) * cost })
		require.NoError(t, d.Rescan())
		watch(t, d)
		from := time.Now()
		for i := 0; time.Since(from) < burst; i++ {
			home.AppendTranscript(sid, cwd, helpers.AssistantReply(sid, fmt.Sprintf("msg_cost_%d", i), stamp(time.Duration(40+i)*time.Second)))
			time.Sleep(every)
		}
		loads := c.since(from)
		require.GreaterOrEqual(t, len(loads), 2)
		shortest := burst
		for i := 1; i < len(loads); i++ {
			shortest = min(shortest, loads[i].Sub(loads[i-1]))
		}

		t.Run("it should wait 3 s between rescans", func(t *testing.T) {
			assert.GreaterOrEqual(t, shortest, 3*time.Second-slack)
		})
	})
}
