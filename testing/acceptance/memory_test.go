//go:build linux

package acceptance_test

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/testing/helpers"
)

// Item 70: the atlas going up at start-up is the largest number the
// city produces. A headless city draws on Mesa's software renderer, here
// and in CI, which keeps every texture in the process's own memory, so
// this measures the software renderer: the atlas and the textures it
// becomes, both counted. Item 61's ruling is to quote Private_Dirty, not
// VmRSS. Before item 70 it peaked at 1.32 GB; after, about 0.46 GB.
//
// Private_Dirty is driver-sized, and pages put up before the loop rather
// than inside it cost only about 60 MB of it, too little to tell from
// another Mesa's noise. Go's own heap tells them apart on any driver:
// its largest size when a collection starts was 346 MB before, 274 MB
// with pages put up before the loop, and 205 MB with the fix.
func TestStartUpPeak(t *testing.T) {
	helpers.NeedXvfb(t)
	const (
		dirtyLimit = 800 << 20
		heapLimit  = 240
	)

	t.Run("when an empty headless city starts and records two seconds", func(t *testing.T) {
		r := newStateRun(t)
		cmd := exec.Command(botropolis, "city", "--socket", "/nonexistent/botropolis.sock", "--headless",
			"--record", filepath.Join(r.tmp, "frames"), "--seconds", "2", "--fps", "5")
		cmd.Env = append(r.env(), "GODEBUG=gctrace=1")
		var trace bytes.Buffer
		cmd.Stderr = &trace
		require.NoError(t, cmd.Start())
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		peak := peakPrivateDirty(t, cmd.Process.Pid, done)
		heap := peakHeapMB(trace.String())
		require.Positive(t, heap, "no collection was traced: %s", trace.String())

		t.Run("it should peak below 800 MB of Private_Dirty", func(t *testing.T) {
			assert.Less(t, peak, int64(dirtyLimit), "peaked at %d MB", peak>>20)
		})

		t.Run("it should start no collection with Go's heap above 240 MB", func(t *testing.T) {
			assert.LessOrEqual(t, heap, heapLimit, "the heap reached %d MB", heap)
		})
	})
}

// gcStart is one gctrace line's heap sizes: at the collection's start,
// at its end, and live after it.
var gcStart = regexp.MustCompile(`(?m)^gc \d+ .* (\d+)->\d+->\d+ MB`)

// peakHeapMB is the largest heap, in MB, that any collection started at.
func peakHeapMB(trace string) int {
	peak := 0
	for _, m := range gcStart.FindAllStringSubmatch(trace, -1) {
		mb, _ := strconv.Atoi(m[1])
		peak = max(peak, mb)
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

func children(parent int) []int {
	entries, _ := os.ReadDir("/proc")
	var out []int
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		stat, err := os.ReadFile(filepath.Join("/proc", e.Name(), "stat"))
		if err != nil {
			continue
		}
		s := string(stat)
		fields := strings.Fields(s[strings.LastIndexByte(s, ')')+1:])
		if len(fields) > 1 && fields[1] == strconv.Itoa(parent) {
			out = append(out, pid)
		}
	}
	return out
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
