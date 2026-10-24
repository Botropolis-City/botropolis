//go:build linux

package acceptance_test

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Item 226: every function of this package's own that reads a process's
// CPU or memory lives here, and tools/acceptance-targets reads this file
// as the performance tier's meters. A test that reaches one of them, or one
// of pkg/measure's, measures, and calls performanceTier(t).

// cpuTicks is the process's user and system time, in clock ticks.
func cpuTicks(t *testing.T, pid int) int64 {
	t.Helper()
	user, system := cpuTicksSplit(t, pid)
	return user + system
}

// cpuTicksSplit is the process's user time and its system time, in the
// kernel, in clock ticks.
func cpuTicksSplit(t *testing.T, pid int) (user, system int64) {
	t.Helper()
	return procTicks(t, pid)
}

// untilIdleMeasured is untilIdle, keeping the peak share of a core and the
// peak Private_Dirty it saw on the way.
func (r *daemonRun) untilIdleMeasured(t *testing.T) {
	t.Helper()
	r.untilQuiet(t, func(share float64) {
		r.peakShare = max(r.peakShare, share)
		r.peakDirty = max(r.peakDirty, privateDirty(r.pid))
	})
}

// threadTicks is the CPU a process's threads have spent, in clock
// ticks: the software rasteriser's (Mesa's llvmpipe threads), and the
// rest.
func threadTicks(t *testing.T, pid int) (raster, rest int64) {
	t.Helper()
	dir := filepath.Join("/proc", strconv.Itoa(pid), "task")
	tasks, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, task := range tasks {
		stat, err := os.ReadFile(filepath.Join(dir, task.Name(), "stat"))
		if err != nil {
			continue
		}
		s := string(stat)
		open, end := strings.IndexByte(s, '('), strings.LastIndexByte(s, ')')
		fields := strings.Fields(s[end+1:])
		user, err := strconv.ParseInt(fields[11], 10, 64)
		require.NoError(t, err)
		system, err := strconv.ParseInt(fields[12], 10, 64)
		require.NoError(t, err)
		if strings.HasPrefix(s[open+1:end], "llvmpipe") {
			raster += user + system
		} else {
			rest += user + system
		}
	}
	return raster, rest
}

// settledPrivateDirty is the largest Private_Dirty of a headless city's
// drawing child over the window before settle.
func settledPrivateDirty(parent int, done chan error, settle, window time.Duration) int64 {
	var peak int64
	start := time.Now()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for time.Since(start) < settle {
		select {
		case err := <-done:
			go func() { done <- err }()
			return peak
		case <-tick.C:
			if time.Since(start) < settle-window {
				continue
			}
			for _, pid := range children(parent) {
				peak = max(peak, privateDirty(pid))
			}
		}
	}
	return peak
}

// peakPrivateDirty samples the drawing child of a headless city, which
// --headless starts under its own virtual display, until the city ends.
func peakPrivateDirty(t *testing.T, parent int, done <-chan error) int64 {
	t.Helper()
	var peak int64
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case err := <-done:
			require.NoError(t, err)
			require.Positive(t, peak, "never saw the drawing process")
			return peak
		case <-tick.C:
			for _, pid := range children(parent) {
				peak = max(peak, privateDirty(pid))
			}
		}
	}
}

func releasedPrivateDirty(pid int, limit time.Duration) (int64, bool) {
	last := privateDirty(pid)
	for deadline := time.Now().Add(limit); time.Now().Before(deadline); {
		time.Sleep(500 * time.Millisecond)
		now := privateDirty(pid)
		if now == last {
			return now, true
		}
		last = now
	}
	return last, false
}

func privateDirty(pid int) int64 {
	f, err := os.Open(filepath.Join("/proc", strconv.Itoa(pid), "smaps_rollup"))
	if err != nil {
		return 0
	}
	defer func() { _ = f.Close() }()
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		if v, ok := strings.CutPrefix(scan.Text(), "Private_Dirty:"); ok {
			kb, _ := strconv.ParseInt(strings.TrimSuffix(strings.TrimSpace(v), " kB"), 10, 64)
			return kb << 10
		}
	}
	return 0
}
