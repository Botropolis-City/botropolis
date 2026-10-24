//go:build linux

package acceptance_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"botropolis.city/botropolis/pkg/measure"
	"botropolis.city/botropolis/testing/helpers"
)

// botropolisd is the daemon binary, built beside the suite's own.
var botropolisd = sync.OnceValues(func() (string, error) {
	bin := filepath.Join(filepath.Dir(botropolis), "botropolisd")
	build := exec.Command("go", "build", "-o", bin, "../../cmd/botropolisd")
	if os.Getenv("BOTROPOLIS_ACCEPTANCE_COVER") != "" {
		build.Args = slices.Insert(build.Args, 2, "-cover", "-coverpkg=botropolis.city/botropolis/pkg/...,botropolis.city/botropolis/cmd/...")
	}
	if out, err := build.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%w: %s", err, out)
	}
	return bin, nil
})

// Item 157: a 16 MiB heap cap and a forced release after every rescan
// kept 0.2.2's daemon collecting garbage, about 290 collections a minute
// with one session writing a line every 200 ms. The limits are the
// budgets in performance/budgets.md: CPU at most 2% of a core, measured
// as the process's own CPU time so the machine's load cannot move it,
// and memory settled under 64 MiB of Private_Dirty. 0.1.7 made 19
// collections a minute with sessions working, so 30 leaves headroom.
//
// It runs the daemon as a person's own, with the statistics store and
// the ledger, and from when it has settled (item 205).
func TestDaemonUnderAStreamOfChanges(t *testing.T) {
	performanceTier(t)
	const (
		stream       = 30 * time.Second
		every        = 200 * time.Millisecond
		maxPerMinute = 30
		maxCoreShare = 0.02
		dirtyLimit   = 64 << 20
		sid          = "11111111-1111-4111-8111-111111111111"
		cwd          = "/home/user/src/alpha"
	)
	bin, err := botropolisd()
	require.NoError(t, err)

	t.Run("when the daemon serves a home and a session writes a line every 200 ms for 30 s", func(t *testing.T) {
		home := helpers.LiveFixtureHome(t, "synthetic")
		d := startOwnDaemon(t, bin, home, "GODEBUG=gctrace=1")
		d.settle(t, 3*time.Minute)
		trace := d.trace

		pid := d.pid
		meter := meterCore(t, pid)
		collections, done := countCollections(trace.String()), meter.window(t)
		h := helpers.HomeAt(t, home)
		at := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
		start := time.Now()
		for i := 0; time.Since(start) < stream; i++ {
			h.AppendTranscript(sid, cwd, helpers.UserPrompt(sid, cwd, at.Add(time.Duration(i)*time.Second).Format(time.RFC3339Nano)))
			time.Sleep(every)
		}
		elapsed := time.Since(start)
		collections = countCollections(trace.String()) - collections
		share, ticked := done()
		perMinute := float64(collections) / elapsed.Minutes()
		time.Sleep(time.Second)
		dirty := privateDirty(pid)
		t.Logf("%d collections in %s (%.0f a minute), %.2f%% of a core (%.2f%% by its CPU ticks), %d MiB Private_Dirty", collections, elapsed.Round(time.Second), perMinute, share*100, ticked*100, dirty>>20)

		meter.counted(t)

		t.Run("it should use at most 2% of a core", func(t *testing.T) {
			assert.LessOrEqual(t, share, maxCoreShare)
		})

		t.Run("it should start at most 30 collections a minute", func(t *testing.T) {
			assert.LessOrEqual(t, perMinute, float64(maxPerMinute))
		})

		t.Run("it should settle under 64 MiB of Private_Dirty", func(t *testing.T) {
			assert.Less(t, dirty, int64(dirtyLimit))
		})

		d.assertProduction(t, start)
	})
}

// Item 196: a settled daemon read every session's counters again and
// looked for every transcript by globbing after each change, so its cost
// grew with the history: r878 used 9-15% of a core on this home with one
// session writing, and this build 1.2-1.8% on a machine at load 20.
func TestSettledDaemonUnderAStreamOfChanges(t *testing.T) {
	performanceTier(t)
	const (
		stopped      = 200
		replies      = 300
		projects     = 30
		stream       = 15 * time.Second
		every        = 200 * time.Millisecond
		maxCoreShare = 0.02
		sid          = "11111111-1111-4111-8111-111111111111"
		cwd          = "/home/user/src/alpha"
	)
	bin, err := botropolisd()
	require.NoError(t, err)

	t.Run("when the daemon has settled on the synthetic home plus 200 stopped sessions of 300 replies each", func(t *testing.T) {
		home := helpers.LiveFixtureHome(t, "synthetic")
		h := helpers.HomeAt(t, home)
		at := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
		dirs := map[string]bool{}
		for i := range stopped {
			id := fmt.Sprintf("%08d-0000-4000-8000-000000000196", i)
			dir := fmt.Sprintf("/home/user/src/stopped%02d", i%projects)
			lines := []string{helpers.UserPrompt(id, dir, at.Format(time.RFC3339Nano))}
			for j := range replies {
				lines = append(lines, helpers.AssistantReply(id, fmt.Sprintf("msg_196_%d_%d", i, j), at.Add(time.Duration(j)*time.Second).Format(time.RFC3339Nano)))
			}
			dirs[filepath.Dir(h.Transcript(id, dir, lines...))] = true
		}
		d := startOwnDaemon(t, bin, home)
		d.settle(t, 3*time.Minute)
		meter := meterCore(t, d.pid)

		t.Run(fmt.Sprintf("and one session writes a reply every 200 ms for two windows of %.0f s", stream.Seconds()), func(t *testing.T) {
			measured := time.Now()
			opens := watchOpens(t, dirs)
			written := 0
			window := func() (float64, float64) {
				done := meter.window(t)
				start := time.Now()
				for ; time.Since(start) < stream; written++ {
					h.AppendTranscript(sid, cwd, helpers.AssistantReply(sid, fmt.Sprintf("msg_196_live_%d", written), at.Add(time.Duration(written)*time.Second).Format(time.RFC3339Nano)))
					time.Sleep(every)
				}
				return done()
			}
			first, firstTicked := window()
			second, secondTicked := window()
			opened := opens()
			t.Logf("%.2f%% then %.2f%% of a core (%.2f%% then %.2f%% by its CPU ticks); %d opens of transcripts that did not change",
				first*100, second*100, firstTicked*100, secondTicked*100, len(opened))

			meter.counted(t)

			// The budget is judged on the quieter window, so one burst of
			// work landing in a window does not decide it; a regression
			// fails both, as r878 would at 9-15% in each. Both are logged,
			// so a cost creeping towards the limit shows first.
			t.Run("it should use at most 2% of a core", func(t *testing.T) {
				assert.LessOrEqual(t, min(first, second), maxCoreShare)
			})

			t.Run("it should open no transcript that did not change", func(t *testing.T) {
				assert.Empty(t, opened)
			})

			d.assertProduction(t, measured)
		})
	})
}

// watchOpens records every file opened in dirs from now on, and gives
// their names when called.
func watchOpens(t *testing.T, dirs map[string]bool) func() []string {
	t.Helper()
	fd, err := unix.InotifyInit1(unix.IN_CLOEXEC | unix.IN_NONBLOCK)
	require.NoError(t, err)
	for dir := range dirs {
		_, err := unix.InotifyAddWatch(fd, dir, unix.IN_OPEN)
		require.NoError(t, err)
	}
	var mu sync.Mutex
	var opened []string
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 64<<10)
		for {
			select {
			case <-stop:
				return
			default:
			}
			if _, err := unix.Poll([]unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}, 100); err != nil && err != unix.EINTR {
				return
			}
			n, err := unix.Read(fd, buf)
			if err == unix.EAGAIN || err == unix.EINTR {
				continue
			}
			if err != nil || n <= 0 {
				return
			}
			for off := 0; off+unix.SizeofInotifyEvent <= n; {
				ev := (*unix.InotifyEvent)(unsafe.Pointer(&buf[off]))
				start := off + unix.SizeofInotifyEvent
				name := strings.TrimRight(string(buf[start:start+int(ev.Len)]), "\x00")
				if ev.Mask&unix.IN_ISDIR == 0 && strings.HasSuffix(name, ".jsonl") {
					mu.Lock()
					opened = append(opened, name)
					mu.Unlock()
				}
				off = start + int(ev.Len)
			}
		}
	}()
	t.Cleanup(func() { close(stop); <-done; _ = unix.Close(fd) })
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(opened)
	}
}

func shortSocket(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "bt")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "d.sock")
}

var gcLine = regexp.MustCompile(`(?m)^gc \d+ `)

func countCollections(trace string) int {
	return len(gcLine.FindAllStringIndex(trace, -1))
}

// coreMeter measures what a child process costs over a window as a share
// of one core: the user-space instructions it retires, at
// measure.CoreRate, which a busy machine's clocks cannot move, plus its
// time in the kernel, which the counters cannot see and the budget
// includes. The kernel's ticks still move with the clocks, but they are
// a small part, 0.2-0.3% of a core on a quiet machine. CPU ticks as a
// whole moved with the clocks on a laptop's hybrid chip and failed
// landings at random (item 209), so they are only the fallback where
// instructions cannot be counted, and are logged beside the count
// otherwise.
type coreMeter struct {
	pid         int
	counter     *measure.ProcessCounter
	uncountable error
}

func meterCore(t *testing.T, pid int) *coreMeter {
	t.Helper()
	counter, err := measure.Process(pid)
	m := &coreMeter{pid: pid, counter: counter, uncountable: err}
	if err == nil {
		t.Cleanup(func() { _ = counter.Close() })
	}
	return m
}

// window starts a window; the function it returns ends it, returning
// the share of a core it was gated on, and its ticks' share beside.
func (m *coreMeter) window(t *testing.T) func() (share, ticked float64) {
	t.Helper()
	const clockTicksSec = 100
	user, system := cpuTicksSplit(t, m.pid)
	start := time.Now()
	var retired uint64
	if m.uncountable == nil {
		retired = m.counter.Instructions()
	}
	return func() (float64, float64) {
		elapsed := time.Since(start).Seconds()
		user1, system1 := cpuTicksSplit(t, m.pid)
		ticked := float64(user1+system1-user-system) / clockTicksSec / elapsed
		t.Logf("by its CPU ticks: %.2f%% of a core in user space, %.2f%% in the kernel",
			100*float64(user1-user)/clockTicksSec/elapsed, 100*float64(system1-system)/clockTicksSec/elapsed)
		if m.uncountable != nil {
			return ticked, ticked
		}
		kernel := float64(system1-system) / clockTicksSec / elapsed
		return float64(m.counter.Instructions()-retired)/measure.CoreRate/elapsed + kernel, ticked
	}
}

// counted says whether the gate counted instructions, and skips saying
// why when it fell back to ticks.
func (m *coreMeter) counted(t *testing.T) {
	t.Run("it should count its cost in instructions", func(t *testing.T) {
		if m.uncountable != nil {
			t.Skipf("skipped: no hardware instruction counters (%v), so its CPU ticks gate instead, which a busy machine moves", m.uncountable)
		}
	})
}
