//go:build linux

package acceptance_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
func TestDaemonUnderAStreamOfChanges(t *testing.T) {
	const (
		stream        = 30 * time.Second
		every         = 200 * time.Millisecond
		maxPerMinute  = 30
		maxCoreShare  = 0.02
		dirtyLimit    = 64 << 20
		clockTicksSec = 100
		sid           = "11111111-1111-4111-8111-111111111111"
		cwd           = "/home/user/src/alpha"
	)
	bin, err := botropolisd()
	require.NoError(t, err)

	t.Run("when the daemon serves a home and a session writes a line every 200 ms for 30 s", func(t *testing.T) {
		root := t.TempDir()
		home := helpers.LiveFixtureHome(t, "synthetic")
		sock := shortSocket(t)
		cmd := exec.Command(bin, "--home", home, "--socket", sock, "--config", filepath.Join(root, "config.toml"))
		cmd.Env = append(os.Environ(), "GODEBUG=gctrace=1", "XDG_STATE_HOME="+filepath.Join(root, "state"), "PATH=/nonexistent")
		var trace syncBuffer
		cmd.Stderr = &trace
		require.NoError(t, cmd.Start())
		t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
		require.Eventually(t, func() bool { _, err := os.Stat(sock); return err == nil }, 10*time.Second, 20*time.Millisecond)
		time.Sleep(2 * time.Second)

		pid := cmd.Process.Pid
		collections, ticks := countCollections(trace.String()), cpuTicks(t, pid)
		h := helpers.HomeAt(t, home)
		at := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
		start := time.Now()
		for i := 0; time.Since(start) < stream; i++ {
			h.AppendTranscript(sid, cwd, helpers.UserPrompt(sid, cwd, at.Add(time.Duration(i)*time.Second).Format(time.RFC3339Nano)))
			time.Sleep(every)
		}
		elapsed := time.Since(start)
		collections = countCollections(trace.String()) - collections
		share := float64(cpuTicks(t, pid)-ticks) / clockTicksSec / elapsed.Seconds()
		perMinute := float64(collections) / elapsed.Minutes()
		time.Sleep(time.Second)
		dirty := privateDirty(pid)
		t.Logf("%d collections in %s (%.0f a minute), %.1f%% of a core, %d MiB Private_Dirty", collections, elapsed.Round(time.Second), perMinute, share*100, dirty>>20)

		t.Run("it should use at most 2% of a core", func(t *testing.T) {
			assert.LessOrEqual(t, share, maxCoreShare)
		})

		t.Run("it should start at most 30 collections a minute", func(t *testing.T) {
			assert.LessOrEqual(t, perMinute, float64(maxPerMinute))
		})

		t.Run("it should settle under 64 MiB of Private_Dirty", func(t *testing.T) {
			assert.Less(t, dirty, int64(dirtyLimit))
		})
	})
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

// cpuTicks is the process's user and system time, in clock ticks.
func cpuTicks(t *testing.T, pid int) int64 {
	t.Helper()
	stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	require.NoError(t, err)
	s := string(stat)
	fields := strings.Fields(s[strings.LastIndexByte(s, ')')+1:])
	user, err := strconv.ParseInt(fields[11], 10, 64)
	require.NoError(t, err)
	system, err := strconv.ParseInt(fields[12], 10, 64)
	require.NoError(t, err)
	return user + system
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
