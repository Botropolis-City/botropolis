package daemon_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"runtime/metrics"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/daemon"
	"botropolis.city/botropolis/pkg/state"
)

const (
	backfillChild  = "BOTROPOLIS_BACKFILL_CHILD"
	debounce       = 100 * time.Millisecond
	backfillBound  = 500 * time.Millisecond
	gaveUp         = 15 * time.Second
	heartbeatEvery = 10 * time.Millisecond
	stoppingMetric = "/sched/pauses/stopping/gc:seconds"
)

var godebugs = []string{"", "asyncpreemptoff=1"}

type cellPriority struct {
	darwinBG bool
	qos      uint32
	diskIO   int
}

var policyPriorities = map[state.BackgroundPolicy]cellPriority{
	state.PolicyNone:               {},
	state.PolicyDarwinBG:           {darwinBG: true},
	state.PolicyQoSUtility:         {qos: 0x11},
	state.PolicyQoSBackground:      {qos: 0x09},
	state.PolicyIOThrottle:         {diskIO: 3},
	state.PolicyDarwinBGIOThrottle: {darwinBG: true, diskIO: 3},
}

// measured is what a child reports for one cell of the matrix.
type measured struct {
	Policy       state.BackgroundPolicy
	GODEBUG      string
	StateChange  time.Duration
	StoppingMax  time.Duration
	StoppingP99  time.Duration
	Pauses       uint64
	HeartbeatGap time.Duration
	Yields       int64
	Before       state.ThreadPriority
	Pass         state.ThreadPriority
}

// slowIngest is a statistics backfill that never ends: it makes garbage
// and burns CPU on the thread the pass gives it, and yields every
// megabyte, the way reading every transcript does, until it is stopped.
// It reads the pass's thread's priority once, from inside the pass.
type slowIngest struct {
	started chan struct{}
	stop    chan struct{}
	once    atomic.Bool
	yields  atomic.Int64
	pass    state.ThreadPriority
	sink    atomic.Pointer[[]byte]
}

func (s *slowIngest) Ingest(_ string, _ time.Time, yield func()) error {
	if s.once.CompareAndSwap(false, true) {
		s.pass = state.ThreadPolicy()
		close(s.started)
	}
	for made := 0; ; made += 8 << 10 {
		select {
		case <-s.stop:
			return nil
		default:
		}
		if made >= 1<<20 {
			yield()
			s.yields.Add(1)
			made = 0
		}
		line := make([]byte, 8<<10)
		for i := range line {
			line[i] = byte(i)
		}
		s.sink.Store(&line)
	}
}

func (s *slowIngest) Hefts() (map[string]claude.Usage, error) {
	return map[string]claude.Usage{}, nil
}

func (s *slowIngest) MovedHefts() (map[string]claude.Usage, []string, error) {
	return map[string]claude.Usage{}, nil, nil
}

// Item 200: item 138's inversion test, ported to macOS, over every
// background policy item 200 weighs. Busy child processes load every
// core, since busy goroutines would be stopped by the collector too.
func TestStateChangeDuringBackfillOnDarwin(t *testing.T) {
	if os.Getenv(backfillChild) != "" {
		measureCell(t, state.BackgroundPolicy(os.Getenv(backfillChild)))
		return
	}
	cpus, err := unix.SysctlUint32("hw.logicalcpu")
	require.NoError(t, err)

	t.Run(fmt.Sprintf("when a slow ingest is in progress, and %d busy child processes load all %d cores", 2*cpus, cpus), func(t *testing.T) {
		for range 2 * cpus {
			loop := exec.Command("sh", "-c", "while :; do :; done")
			require.NoError(t, loop.Start())
			t.Cleanup(func() {
				_ = loop.Process.Kill()
				_ = loop.Wait()
			})
		}
		for _, policy := range state.BackgroundPolicies {
			for _, godebug := range godebugs {
				t.Run(fmt.Sprintf("and the pass runs under %s with GODEBUG %q", policy, godebug), func(t *testing.T) {
					m := runCell(t, policy, godebug)
					t.Logf("item 200 cell: policy=%s godebug=%q state-change=%v stopping-max=%v stopping-p99=%v pauses=%d heartbeat-gap=%v yields=%d before=%+v pass=%+v",
						m.Policy, m.GODEBUG, m.StateChange, m.StoppingMax, m.StoppingP99, m.Pauses, m.HeartbeatGap, m.Yields, m.Before, m.Pass)
					want := policyPriorities[policy]

					t.Run("it should apply the policy's PRIO_DARWIN_BG flag to the pass's thread", func(t *testing.T) {
						assert.Equal(t, want.darwinBG, m.Pass.DarwinBG)
					})

					t.Run("it should apply the policy's QoS class to the pass's thread", func(t *testing.T) {
						assert.Equal(t, cmp(want.qos, m.Before.QoS), m.Pass.QoS)
					})

					t.Run("it should apply the policy's disk I/O policy to the pass's thread", func(t *testing.T) {
						assert.Equal(t, cmp(want.diskIO, m.Before.DiskIO), m.Pass.DiskIO)
					})

					t.Run("it should make progress in the ingest", func(t *testing.T) {
						assert.Positive(t, m.Yields)
					})
				})
			}
		}
	})
}

// cmp is the policy's value, or the thread's own when the policy leaves
// it alone.
func cmp[T comparable](want, before T) T {
	var zero T
	if want == zero {
		return before
	}
	return want
}

func runCell(t *testing.T, policy state.BackgroundPolicy, godebug string) measured {
	t.Helper()
	var out bytes.Buffer
	child := exec.Command(os.Args[0], "-test.run", "^TestStateChangeDuringBackfillOnDarwin$", "-test.count", "1", "-test.v")
	child.Env = append(os.Environ(), backfillChild+"="+string(policy))
	if godebug != "" {
		child.Env = append(child.Env, "GODEBUG="+godebug)
	}
	child.Stdout, child.Stderr = &out, &out
	require.NoError(t, child.Run(), out.String())
	match := regexp.MustCompile(`item 200 measured: (\{.*\})`).FindStringSubmatch(out.String())
	require.NotNil(t, match, out.String())
	var m measured
	require.NoError(t, json.Unmarshal([]byte(match[1]), &m))
	m.GODEBUG = godebug
	return m
}

// measureCell runs in the child: the backfill under policy, a session's
// change of state timed against it, the collector's stopping pauses and
// a heartbeat's largest gap over the same window.
func measureCell(t *testing.T, policy state.BackgroundPolicy) {
	state.SetBackgroundPolicy(policy)
	m := measured{Policy: policy}
	m.Before = onOwnThread(state.ThreadPolicy)

	home := idleHome(t, "interactive")
	ingest := &slowIngest{started: make(chan struct{}), stop: make(chan struct{})}
	t.Cleanup(func() { close(ingest.stop) })
	loader := state.NewLoader(home.Path, probes).WithStats(ingest, nil)
	d := daemon.NewWith(loader, func() time.Time { return now })
	require.NoError(t, d.Rescan())
	<-ingest.started
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = d.Watch(ctx) }()
	require.NoError(t, d.WaitWatching(ctx))
	runtime.Gosched()
	time.Sleep(time.Second)

	pausesBefore := stoppingPauses()
	yieldsBefore := ingest.yields.Load()
	var gap atomic.Int64
	beat, beating := context.WithCancel(ctx)
	go heartbeat(beat, &gap)

	start := time.Now()
	home.Session(pid, sid, cwd, "interactive", "busy")
	for time.Since(start) < gaveUp {
		if s, ok := session(d, sid); ok && s.State == state.Working {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	m.StateChange = time.Since(start)
	time.Sleep(2 * time.Second)
	beating()

	m.HeartbeatGap = time.Duration(gap.Load())
	m.Yields = ingest.yields.Load() - yieldsBefore
	m.Pass = ingest.pass
	m.Pauses, m.StoppingMax, m.StoppingP99 = pausesSince(pausesBefore)
	line, err := json.Marshal(m)
	require.NoError(t, err)
	t.Logf("item 200 measured: %s", line)
}

func onOwnThread[T any](f func() T) T {
	got := make(chan T)
	go func() {
		runtime.LockOSThread()
		got <- f()
	}()
	return <-got
}

func heartbeat(ctx context.Context, gap *atomic.Int64) {
	tick := time.NewTicker(heartbeatEvery)
	defer tick.Stop()
	last := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tick.C:
			if g := int64(now.Sub(last)); g > gap.Load() {
				gap.Store(g)
			}
			last = now
		}
	}
}

func stoppingPauses() *metrics.Float64Histogram {
	s := []metrics.Sample{{Name: stoppingMetric}}
	metrics.Read(s)
	return s[0].Value.Float64Histogram()
}

// pausesSince is how many stopping pauses there were since before, and
// the largest and the 99th percentile of them, each the upper bound of
// its bucket.
func pausesSince(before *metrics.Float64Histogram) (count uint64, largest, p99 time.Duration) {
	after := stoppingPauses()
	counts := make([]uint64, len(after.Counts))
	for i := range counts {
		counts[i] = after.Counts[i] - before.Counts[i]
		count += counts[i]
	}
	bound := func(i int) time.Duration {
		b := after.Buckets[i+1]
		if math.IsInf(b, 1) {
			b = after.Buckets[i]
		}
		return time.Duration(b * float64(time.Second))
	}
	var seen uint64
	for i, c := range counts {
		if c == 0 {
			continue
		}
		largest = bound(i)
		seen += c
		if p99 == 0 && float64(seen) >= 0.99*float64(count) {
			p99 = bound(i)
		}
	}
	return count, largest, p99
}
