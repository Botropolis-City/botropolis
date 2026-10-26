package daemon_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/daemon"
	"botropolis.city/botropolis/pkg/measure"
	"botropolis.city/botropolis/pkg/state"
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

// stepClock is the watch's clock for a test: it never moves on its own,
// hands the test every wait the watch asks for, and fires when told.
type stepClock struct {
	asked chan time.Duration
	fire  chan time.Time
}

func newStepClock() *stepClock {
	return &stepClock{asked: make(chan time.Duration, 8), fire: make(chan time.Time)}
}

func (c *stepClock) Now() time.Time { return now }

func (c *stepClock) After(d time.Duration) <-chan time.Time {
	c.asked <- d
	return c.fire
}

// next is the next wait the watch asks for.
func (c *stepClock) next(t *testing.T) time.Duration {
	t.Helper()
	select {
	case d := <-c.asked:
		return d
	case <-time.After(10 * time.Second):
		require.FailNow(t, "the watch never asked to wait")
		return 0
	}
}

// Item 213: the pace holds rescans to 1% of a core as the cost gates
// count a core, the instructions a rescan retires at measure.CoreRate
// and its kernel time; a rescan's thread CPU, which ran a third or more
// under that on a quiet machine, paces them only where nothing can be
// counted.
func TestWatchPacesByCost(t *testing.T) {
	const counted30ms = 0.030 * measure.CoreRate
	paced := func(t *testing.T, cost measure.Cost) (*daemon.Daemon, *loadClock, *stepClock) {
		home := idleHome(t, "interactive")
		c := &loadClock{Loader: state.NewLoader(home.Path, probes)}
		d := daemon.NewWith(c, func() time.Time { return now })
		daemon.SetRescanMeter(d, func(rescan func()) measure.Cost { rescan(); return cost })
		clock := newStepClock()
		daemon.SetWatchClock(d, clock)
		watch(t, d)
		d.RequestRescan()
		clock.next(t)
		clock.fire <- now
		return d, c, clock
	}
	cases := []struct {
		scenario string
		cost     measure.Cost
		want     time.Duration
	}{
		{scenario: "when every rescan counts 30 ms of a core but uses 15 ms of thread CPU",
			cost: measure.Cost{CPU: 15 * time.Millisecond, Instructions: uint64(counted30ms), Counted: true}, want: 3 * time.Second},
		{scenario: "when the rescans' instructions cannot be counted and each uses 30 ms of thread CPU",
			cost: measure.Cost{CPU: 30 * time.Millisecond}, want: 3 * time.Second},
	}
	for _, c := range cases {
		t.Run(c.scenario+", and a change comes right after a rescan", func(t *testing.T) {
			d, _, clock := paced(t, c.cost)
			d.RequestRescan()
			waited := clock.next(t)

			t.Run(fmt.Sprintf("it should wait %v for the next rescan", c.want), func(t *testing.T) {
				assert.InDelta(t, c.want, waited, float64(time.Millisecond))
			})
		})
	}

	t.Run("when every rescan uses 60 ms, so the watch waits its 5 s cap, and a hook event arrives between rescans", func(t *testing.T) {
		d, loads, clock := paced(t, measure.Cost{CPU: 60 * time.Millisecond})
		d.RequestRescan()
		waited := clock.next(t)
		updates, cancel := d.Subscribe()
		t.Cleanup(cancel)
		before := len(loads.since(time.Time{}))
		d.Apply(claude.HookEvent{Name: claude.HookPreToolUse, SessionID: sid, ToolName: "Bash"}, now.Add(time.Hour))
		var got state.Session
		select {
		case u := <-updates:
			for _, s := range u.Snapshot.Sessions {
				if s.ID == sid {
					got = s
				}
			}
		case <-time.After(10 * time.Second):
			require.FailNow(t, "no update reached the subscriber")
		}
		require.Equal(t, 5*time.Second, waited, "the watch did not wait its cap")

		t.Run("it should hand subscribers the hook's state", func(t *testing.T) {
			assert.Equal(t, state.Working, got.State)
		})

		t.Run("it should hand it over before the next rescan", func(t *testing.T) {
			assert.Equal(t, before, len(loads.since(time.Time{})))
		})
	})
}
