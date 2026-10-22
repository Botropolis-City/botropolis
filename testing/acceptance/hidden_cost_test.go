//go:build linux

package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/render"
	"botropolis.city/botropolis/pkg/xvfb"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 197: a city nobody was watching laid the whole city out again
// for every snapshot the daemon sent, at 7.9% of a core on the real
// history against the budget in performance/budgets.md: hidden,
// minimised or unfocused, at most 1% of one core. A hidden city now
// holds its snapshots, and lets its subscription go but for a glance
// every 5 s, because decoding the whole snapshot each second was most
// of what was left; an unfocused one that can be seen is a dashboard
// (Aria, 2026-10-21), laying out each snapshot and painting one frame
// for it, against the same 1%.
//
// The city runs on a virtual display against a daemon of 200 sessions
// in 150 projects while one session writes a line every 200 ms, and its
// own process CPU over 30 s is the measure, so the machine's load
// cannot move it. The frame timer says what the window manager said,
// because a window in another state would be measuring something else.
func TestAnUnwatchedCityUnderAStreamOfSnapshots(t *testing.T) {
	const (
		settle        = 20 * time.Second
		stream        = 30 * time.Second
		every         = 200 * time.Millisecond
		maxCoreShare  = 0.01
		metBelow      = 0.008
		clockTicksSec = 100
	)
	for _, way := range []struct {
		name string
		do   func(testing.TB, *xvfb.Server)
		// said is what the frame timer reports while the window is so.
		said string
		// drawn is whether the city paints while so: a virtual display
		// paints in software, on the rasteriser's own threads, which a
		// graphics card does instead, so what is gated is the rest.
		drawn bool
		// unmet names what is to bring the case under 1%, while it is
		// not: the case skips, printing the gap, and fails once it is
		// met, as item 194's targets do.
		unmet string
		// fed is the least it should read of the daemon: a glance of
		// about 220 KiB every 5 s while it cannot be seen, and while it
		// can, what changed, about a second apart (item 201).
		fed   int64
		taken string
	}{
		{"hidden", helpers.Hide, "visible=false", false, "", 512 << 10, "at a glance every 5 s"},
		{"unfocused, and visible", helpers.Unfocus, "focused=false visible=true minimised=false", true, "item 201", 4 << 10, "as what changed, about once a second"},
	} {
		t.Run("when the city is "+way.name+", and a session of 200 writes a line every 200 ms for 30 s", func(t *testing.T) {
			b := newBusyCity(t)
			root := b.root

			display := helpers.Display(t, render.DefaultWidth, render.DefaultHeight)
			city := exec.Command(botropolis, "city", "--home", b.home.Path, "--socket", b.sock, "--config", b.config())
			city.Env = append(cleanEnv(), "HOME="+filepath.Join(root, "home"), "XDG_STATE_HOME="+filepath.Join(root, "city-state"), "TMPDIR="+root, "BOTROPOLIS_FRAMETIME=1")
			city.Env = append(city.Env, display.Env()...)
			var said syncBuffer
			city.Stderr = &said
			require.NoError(t, city.Start())
			t.Cleanup(func() { _ = city.Process.Kill(); _ = city.Wait() })
			time.Sleep(settle)
			way.do(t, display)
			time.Sleep(2 * time.Second)

			heard := len(said.String())
			raster, rest := threadTicks(t, city.Process.Pid)
			read := bytesRead(t, city.Process.Pid)
			elapsed := b.write(every, stream)
			raster1, rest1 := threadTicks(t, city.Process.Pid)
			share := func(ticks int64) float64 { return float64(ticks) / clockTicksSec / elapsed.Seconds() }
			rasterShare, restShare := share(raster1-raster), share(rest1-rest)
			fed := bytesRead(t, city.Process.Pid) - read
			during := said.String()[heard:]
			t.Logf("city %s: %.2f%% of a core over %.0f s outside the software rasteriser, %.2f%% in it, %d KiB of snapshots read",
				way.name, 100*restShare, elapsed.Seconds(), 100*rasterShare, fed>>10)
			measured := restShare + rasterShare
			if way.drawn {
				measured = restShare
			}

			t.Run("the daemon's snapshots should have reached it, "+way.taken, func(t *testing.T) {
				assert.Greater(t, fed, way.fed)
			})

			t.Run("the window should have been "+way.name+" throughout", func(t *testing.T) {
				require.Contains(t, during, "QUIET no frames: ")
				for _, line := range strings.Split(during, "\n") {
					if strings.HasPrefix(line, "QUIET no frames: ") && !strings.Contains(line, way.said) {
						t.Fatalf("the window was not %s: %s", way.name, line)
					}
				}
			})

			t.Run("its CPU, but for drawing in software, should be at most 1% of a core", func(t *testing.T) {
				if way.unmet != "" {
					// Runs differ by half a percent, so only a clear pass says
					// the target is met; one that lands under it by luck skips.
					require.Greater(t, measured, metBelow, "target met: remove the skip")
					t.Skipf("unmet: %.2f%% against 1%%; %s", 100*measured, way.unmet)
				}
				assert.LessOrEqual(t, measured, maxCoreShare)
			})
		})
	}
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

// cleanEnv is the test's environment without anything naming a display
// or a home, which each run sets for itself.
func cleanEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		switch key {
		case "HOME", "XDG_STATE_HOME", "XDG_CONFIG_HOME", "TMPDIR", "DISPLAY", "WAYLAND_DISPLAY", "XAUTHORITY":
		default:
			env = append(env, kv)
		}
	}
	return env
}
