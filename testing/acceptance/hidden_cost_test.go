//go:build linux

package acceptance_test

import (
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

	"botropolis.city/botropolis/pkg/render"
	"botropolis.city/botropolis/pkg/xvfb"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 197: a city nobody was watching laid the whole city out again
// for every snapshot the daemon sent, at 7.9% of a core on the real
// history against the budget in performance/budgets.md: hidden,
// minimised or unfocused, at most 1% of one core. A hidden city now
// holds its snapshots, and lets its subscription go but for a glance
// every 5 s; an unfocused one that can be seen is a dashboard (Aria,
// 2026-10-21), laying out each snapshot and painting one frame for it.
//
// The city runs on a virtual display against item 197's busy city while
// one session writes a line every 200 ms. A hidden city draws nothing,
// so its own process CPU is the measure, which the machine's load
// cannot move. An unfocused one draws, and a virtual display draws in
// software, so its process CPU says more about Mesa than about the
// city. **Aria's 1% for the dashboard is the whole process, drawing
// included**, and drawing cannot be measured meaningfully here: so
// what is gated is the time the city itself counts decoding the
// daemon's updates and laying them out (item 201), with the process's
// CPU logged beside it on every run, and the whole process is measured
// on real hardware by the benchmark session. The frame timer says what
// the window manager said, because a window in another state would be
// measuring something else.
func TestAnUnwatchedCityUnderAStreamOfSnapshots(t *testing.T) {
	const (
		stream        = 15 * time.Second
		every         = 200 * time.Millisecond
		maxCoreShare  = 0.01
		clockTicksSec = 100
	)
	for _, way := range []struct {
		name string
		do   func(testing.TB, *xvfb.Server)
		// said is what the frame timer reports while the window is so.
		said string
		// drawn is whether the city paints while so; when it does, the
		// time it counts decoding and laying out is what is gated.
		drawn bool
		// fed is the least it should read of the daemon while hidden: a
		// glance of about 220 KiB every 5 s.
		fed int64
	}{
		{"hidden", helpers.Hide, "visible=false", false, 256 << 10},
		{"unfocused, and visible", helpers.Unfocus, "focused=false visible=true minimised=false", true, 0},
	} {
		t.Run("when the city is "+way.name+", and a session of 200 writes a line every 200 ms for 15 s", func(t *testing.T) {
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
			// The city is running once its frame timer speaks, which it
			// does whether it draws or not.
			require.Eventually(t, func() bool {
				s := said.String()
				return strings.Contains(s, "QUIET ") || strings.Contains(s, "FRAME ")
			}, 60*time.Second, 100*time.Millisecond)
			time.Sleep(3 * time.Second)
			way.do(t, display)
			if way.drawn {
				require.Eventually(t, func() bool { c, ok := lastQuiet(said.String()); return ok && c.snapshots > 0 }, 30*time.Second, 100*time.Millisecond)
			}
			time.Sleep(2 * time.Second)

			heard := len(said.String())
			counted, _ := lastQuiet(said.String())
			raster, rest := threadTicks(t, city.Process.Pid)
			read := bytesRead(t, city.Process.Pid)
			elapsed := b.write(every, stream)
			raster1, rest1 := threadTicks(t, city.Process.Pid)
			fed := bytesRead(t, city.Process.Pid) - read
			time.Sleep(1500 * time.Millisecond)
			counted1, _ := lastQuiet(said.String())
			share := func(ticks int64) float64 { return float64(ticks) / clockTicksSec / elapsed.Seconds() }
			rasterShare, restShare := share(raster1-raster), share(rest1-rest)
			own := (counted1.decoding + counted1.layout - counted.decoding - counted.layout).Seconds() / elapsed.Seconds()
			during := said.String()[heard:]
			t.Logf("city %s: decoding and laying out %d snapshots %.2f%% of a core over %.0f s (decoding %.0f ms, laying out %.0f ms); process %.2f%% outside the software rasteriser, %.2f%% in it; %d KiB read",
				way.name, counted1.snapshots-counted.snapshots, 100*own, elapsed.Seconds(),
				ms(counted1.decoding-counted.decoding), ms(counted1.layout-counted.layout), 100*restShare, 100*rasterShare, fed>>10)
			measured := restShare + rasterShare
			if way.drawn {
				measured = own
			}

			t.Run("the daemon's updates should have reached it", func(t *testing.T) {
				if way.drawn {
					assert.Greater(t, counted1.snapshots-counted.snapshots, 5)
				} else {
					assert.Greater(t, fed, way.fed)
				}
			})

			t.Run("the window should have been "+way.name+" throughout", func(t *testing.T) {
				require.Contains(t, during, "QUIET no frames: ")
				for _, line := range strings.Split(during, "\n") {
					if strings.HasPrefix(line, "QUIET no frames: ") && !strings.Contains(line, way.said) {
						t.Fatalf("the window was not %s: %s", way.name, line)
					}
				}
			})

			t.Run("its CPU, but for drawing, should be at most 1% of a core", func(t *testing.T) {
				assert.LessOrEqual(t, measured, maxCoreShare)
			})
		})
	}
}

// quietCount is what the frame timer's last QUIET line said the city's
// snapshots have cost it.
type quietCount struct {
	snapshots        int
	decoding, layout time.Duration
}

func ms(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000
}

func lastQuiet(said string) (quietCount, bool) {
	lines := strings.Split(said, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		at := strings.Index(lines[i], " snapshots=")
		if !strings.HasPrefix(lines[i], "QUIET no frames: ") || at < 0 {
			continue
		}
		var c quietCount
		var decoding, layout float64
		if _, err := fmt.Sscanf(lines[i][at:], " snapshots=%d decoding=%fms layout=%fms", &c.snapshots, &decoding, &layout); err != nil {
			continue
		}
		c.decoding = time.Duration(decoding * float64(time.Millisecond))
		c.layout = time.Duration(layout * float64(time.Millisecond))
		return c, true
	}
	return quietCount{}, false
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
