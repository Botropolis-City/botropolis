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
		maxCores      = 1.0
		dirtyLimit    = 256 << 20
		clockTicksSec = 100
		idleShare     = 0.05
		idleSamples   = 3
		gaveUp        = 10 * time.Minute
	)
	history := int64(256 << 20)
	if gb, err := strconv.ParseFloat(os.Getenv(firstLaunchGB), 64); err == nil && gb > 0 {
		history = int64(gb * (1 << 30))
	}
	bin, err := botropolisd()
	require.NoError(t, err)

	t.Run(fmt.Sprintf("when the daemon starts on an empty state directory over %d MiB of history", history>>20), func(t *testing.T) {
		root := t.TempDir()
		home := filepath.Join(root, "home")
		written := syntheticHistory(t, helpers.HomeAt(t, home), history)
		sock := shortSocket(t)
		cmd := exec.Command(bin, "--socket", sock, "--config", filepath.Join(root, "config.toml"))
		cmd.Env = append(os.Environ(), "HOME="+home, "XDG_STATE_HOME="+filepath.Join(root, "state"), "PATH=/nonexistent")
		require.NoError(t, cmd.Start())
		t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
		pid := cmd.Process.Pid

		var peakShare float64
		var peakDirty int64
		quiet := 0
		start := time.Now()
		last, at := cpuTicks(t, pid), time.Now()
		for quiet < idleSamples && time.Since(start) < gaveUp {
			time.Sleep(time.Second)
			ticks, now := cpuTicks(t, pid), time.Now()
			share := float64(ticks-last) / clockTicksSec / now.Sub(at).Seconds()
			last, at = ticks, now
			peakShare = max(peakShare, share)
			peakDirty = max(peakDirty, privateDirty(pid))
			if share < idleShare {
				quiet++
			} else {
				quiet = 0
			}
		}
		read := bytesRead(t, pid)
		t.Logf("%d MiB of history in %s: peak %.0f%% of a core, peak %d MiB Private_Dirty, %d MiB read",
			written>>20, time.Since(start).Round(time.Second), peakShare*100, peakDirty>>20, read>>20)

		t.Run("it should read the whole history", func(t *testing.T) {
			assert.GreaterOrEqual(t, read, written)
		})

		t.Run("it should use at most one core in any 1 s sample", func(t *testing.T) {
			assert.LessOrEqual(t, peakShare, maxCores)
		})

		t.Run("it should stay under 256 MiB of Private_Dirty at every sample", func(t *testing.T) {
			assert.Less(t, peakDirty, int64(dirtyLimit))
		})
	})
}

// syntheticHistory writes stopped sessions under home until they hold
// size bytes, each turn a prompt carrying a tool's worth of output and
// a reply with usage, the shape that makes real transcripts large. It
// returns how many bytes it wrote.
func syntheticHistory(t *testing.T, home *helpers.Home, size int64) int64 {
	t.Helper()
	const (
		perSession = 8 << 20
		cwd        = "/home/user/src/history"
	)
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
