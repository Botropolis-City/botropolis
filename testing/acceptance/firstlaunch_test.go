//go:build linux

package acceptance_test

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/stats"
	"botropolis.city/botropolis/testing/helpers"
)

const firstLaunchGB = "BOTROPOLIS_FIRST_LAUNCH_GB"

// Item 159: 0.2.2's first launch took up to four cores, and 0.2.3's a
// peak of 414%. The limits are the budget's (performance/budgets.md,
// First launch): at most one core in any 1 s sample, at background
// priority, and at most 256 MiB of Private_Dirty at every sample. Both
// are per sample, so the 256 MiB history the suite builds measures the
// same rate a large one would, only for less time; set
// BOTROPOLIS_FIRST_LAUNCH_GB=3 to run it at the item's size for a
// report.
func TestFirstLaunch(t *testing.T) {
	const (
		maxCores   = 1.0
		dirtyLimit = 256 << 20
	)
	history := int64(256 << 20)
	if gb, err := strconv.ParseFloat(os.Getenv(firstLaunchGB), 64); err == nil && gb > 0 {
		history = int64(gb * (1 << 30))
	}

	t.Run(fmt.Sprintf("when the daemon starts on an empty state directory over %d MiB of history", history>>20), func(t *testing.T) {
		l := newLaunch(t, history, 8<<20)
		run := l.start(t)
		run.untilIdle(t)
		read := bytesRead(t, run.pid)
		t.Logf("%d MiB of history in %s: peak %.0f%% of a core, peak %d MiB Private_Dirty, %d MiB read",
			l.written>>20, run.took.Round(time.Second), run.peakShare*100, run.peakDirty>>20, read>>20)

		t.Run("it should read the whole history", func(t *testing.T) {
			assert.GreaterOrEqual(t, read, l.written)
		})

		t.Run("it should use at most one core in any 1 s sample", func(t *testing.T) {
			assert.LessOrEqual(t, run.peakShare, maxCores)
		})

		t.Run("it should stay under 256 MiB of Private_Dirty at every sample", func(t *testing.T) {
			assert.Less(t, run.peakDirty, int64(dirtyLimit))
		})
	})
}

// Item 159: on the 265U, 0.2.2 stopped 6 s into a first launch read
// 89% of the history again when started on the same state. The
// transcripts here are longer than a checkpoint, so a stop lands inside
// one; the daemon is killed rather than asked to quit, so nothing but
// the checkpoints can save it.
func TestFirstLaunchResumes(t *testing.T) {
	const (
		history    = 96 << 20
		perSession = 48 << 20
		stopAt     = 40 << 20
	)

	t.Run("when the daemon is killed part-way through its first launch and started again on the same state", func(t *testing.T) {
		whole := newLaunch(t, history, perSession)
		uninterrupted := whole.start(t)
		uninterrupted.untilIdle(t)
		once := bytesRead(t, uninterrupted.pid)
		again := once - whole.written

		l := newLaunch(t, history, perSession)
		first := l.start(t)
		require.Eventually(t, func() bool { return bytesRead(t, first.pid) >= stopAt }, 5*time.Minute, 50*time.Millisecond)
		first.kill(t)
		before := first.read
		second := l.start(t)
		second.untilIdle(t)
		total := before + bytesRead(t, second.pid)
		t.Logf("one launch read %d MiB of %d; killed at %d MiB and started again, %d MiB in all", once>>20, whole.written>>20, before>>20, total>>20)

		t.Run("it should read no more than one launch, one checkpoint, and a second start's own reads", func(t *testing.T) {
			assert.LessOrEqual(t, total, once+stats.CheckpointEvery+again)
		})
	})
}

// launch is a home of synthetic history with its own state directory.
type launch struct {
	root, home string
	written    int64
}

func newLaunch(t *testing.T, history, perSession int64) *launch {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	return &launch{root: root, home: home, written: syntheticHistory(t, helpers.HomeAt(t, home), history, perSession)}
}

// daemonRun is one daemon started on a launch, and what it was sampled
// at.
type daemonRun struct {
	cmd       *exec.Cmd
	pid       int
	read      int64
	peakShare float64
	peakDirty int64
	took      time.Duration
	killed    bool
}

func (l *launch) start(t *testing.T) *daemonRun {
	t.Helper()
	bin, err := botropolisd()
	require.NoError(t, err)
	cmd := exec.Command(bin, "--socket", shortSocket(t), "--config", filepath.Join(l.root, "config.toml"))
	cmd.Env = append(os.Environ(), "HOME="+l.home, "XDG_STATE_HOME="+filepath.Join(l.root, "state"), "PATH=/nonexistent")
	require.NoError(t, cmd.Start())
	r := &daemonRun{cmd: cmd, pid: cmd.Process.Pid}
	t.Cleanup(func() {
		if !r.killed {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	return r
}

// kill stops the daemon at once, noting what it had read.
func (r *daemonRun) kill(t *testing.T) {
	t.Helper()
	r.read = bytesRead(t, r.pid)
	require.NoError(t, r.cmd.Process.Kill())
	_ = r.cmd.Wait()
	r.killed = true
}

// untilIdle samples the daemon each second until it has used under 5%
// of a core for three samples running.
func (r *daemonRun) untilIdle(t *testing.T) {
	t.Helper()
	const (
		clockTicksSec = 100
		idleShare     = 0.05
		idleSamples   = 3
		gaveUp        = 10 * time.Minute
	)
	quiet := 0
	start := time.Now()
	last, at := cpuTicks(t, r.pid), time.Now()
	for quiet < idleSamples && time.Since(start) < gaveUp {
		time.Sleep(time.Second)
		ticks, now := cpuTicks(t, r.pid), time.Now()
		share := float64(ticks-last) / clockTicksSec / now.Sub(at).Seconds()
		last, at = ticks, now
		r.peakShare = max(r.peakShare, share)
		r.peakDirty = max(r.peakDirty, privateDirty(r.pid))
		if share < idleShare {
			quiet++
		} else {
			quiet = 0
		}
	}
	r.took = time.Since(start)
}

// syntheticHistory writes stopped sessions under home until they hold
// size bytes, each at most perSession, each turn a prompt carrying a
// tool's worth of output and a reply with usage, the shape that makes
// real transcripts large. It returns how many bytes it wrote.
func syntheticHistory(t *testing.T, home *helpers.Home, size, perSession int64) int64 {
	t.Helper()
	const cwd = "/home/user/src/history"
	output := strings.Repeat("x", 6<<10)
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var written int64
	for s := 0; written < size; s++ {
		sid := fmt.Sprintf("%08x-0000-4000-8000-%012x", s, s)
		path := home.Transcript(sid, cwd)
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
		require.NoError(t, err)
		w := bufio.NewWriterSize(f, 1<<20)
		var inSession int64
		for turn := 0; inSession < perSession && written+inSession < size; turn++ {
			ts := at.Add(time.Duration(s)*time.Hour + time.Duration(turn)*time.Second).Format(time.RFC3339Nano)
			prompt := strings.Replace(helpers.UserPrompt(sid, cwd, ts), `"content":"hi"`, `"content":"`+output+`"`, 1)
			reply := helpers.AssistantReply(sid, fmt.Sprintf("msg_%d_%d", s, turn), ts)
			n, err := fmt.Fprintf(w, "%s\n%s\n", prompt, reply)
			require.NoError(t, err)
			inSession += int64(n)
		}
		require.NoError(t, w.Flush())
		require.NoError(t, f.Close())
		written += inSession
	}
	return written
}

// bytesRead is what the process has read, from any file, in bytes.
func bytesRead(t *testing.T, pid int) int64 {
	t.Helper()
	io, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "io"))
	require.NoError(t, err)
	for line := range strings.SplitSeq(string(io), "\n") {
		if v, ok := strings.CutPrefix(line, "rchar: "); ok {
			n, err := strconv.ParseInt(v, 10, 64)
			require.NoError(t, err)
			return n
		}
	}
	require.FailNow(t, "no rchar in /proc/pid/io")
	return 0
}
