package daemon_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
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
	backfillChild = "BOTROPOLIS_BACKFILL_CHILD"
	// debounce is the watcher's; the bound is what a state change may
	// take beyond it while statistics are being caught up.
	debounce      = 100 * time.Millisecond
	backfillBound = 500 * time.Millisecond
	busyLoops     = 4
	gaveUp        = 15 * time.Second
)

// slowIngest is a statistics backfill that never ends: it makes garbage
// and burns CPU on the thread the pass gives it, and yields every
// megabyte, the way reading every transcript does, until it is stopped.
type slowIngest struct {
	started chan struct{}
	stop    chan struct{}
	once    atomic.Bool
	sink    atomic.Pointer[[]byte]
}

func (s *slowIngest) Ingest(_ string, _ time.Time, yield func()) error {
	if s.once.CompareAndSwap(false, true) {
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

// Item 138: while the statistics were backfilled on a busy machine, a
// session's change of state reached the map 6-15 s late.
func TestStateChangeDuringBackfill(t *testing.T) {
	if os.Getenv(backfillChild) != "" {
		measureStateChange(t)
		return
	}
	taskset, err := exec.LookPath("taskset")
	if err != nil {
		t.Skip("taskset is not installed, so no core can be made busy for this measure")
	}

	t.Run("when a slow ingest is in progress, and every CPU the daemon may use is busy", func(t *testing.T) {
		core := firstAllowedCore(t)
		for range busyLoops {
			loop := exec.Command(taskset, "-c", core, "sh", "-c", "while :; do :; done")
			require.NoError(t, loop.Start())
			t.Cleanup(func() {
				_ = loop.Process.Kill()
				_ = loop.Wait()
			})
		}
		var out bytes.Buffer
		child := exec.Command(taskset, "-c", core, os.Args[0], "-test.run", "^TestStateChangeDuringBackfill$", "-test.count", "1", "-test.v")
		child.Env = append(os.Environ(), backfillChild+"=1")
		child.Stdout, child.Stderr = &out, &out
		require.NoError(t, child.Run(), out.String())
		match := regexp.MustCompile(`state change took (\d+) ns`).FindStringSubmatch(out.String())
		require.NotNil(t, match, out.String())
		took, err := strconv.ParseInt(match[1], 10, 64)
		require.NoError(t, err)
		t.Logf("a state change took %v beside the backfill", time.Duration(took))

		t.Run("and a session file changes to busy", func(t *testing.T) {
			t.Run(fmt.Sprintf("it should reach the snapshot within the debounce plus %v", backfillBound), func(t *testing.T) {
				assert.LessOrEqual(t, time.Duration(took), debounce+backfillBound)
			})
		})
	})
}

// firstAllowedCore is the lowest CPU this process may run on, so a
// runner that withholds core 0 still has one to make busy.
func firstAllowedCore(t *testing.T) string {
	t.Helper()
	var set unix.CPUSet
	require.NoError(t, unix.SchedGetaffinity(0, &set))
	for cpu := range 1024 {
		if set.IsSet(cpu) {
			return strconv.Itoa(cpu)
		}
	}
	require.FailNow(t, "this process may run on no CPU")
	return ""
}

func measureStateChange(t *testing.T) {
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

	start := time.Now()
	home.Session(pid, sid, cwd, "interactive", "busy")
	for time.Since(start) < gaveUp {
		if s, ok := session(d, sid); ok && s.State == state.Working {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Logf("state change took %d ns", time.Since(start).Nanoseconds())
}
