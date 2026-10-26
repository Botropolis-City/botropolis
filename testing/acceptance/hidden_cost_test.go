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

	"botropolis.city/botropolis/pkg/measure"
	"botropolis.city/botropolis/pkg/render"
	"botropolis.city/botropolis/pkg/xvfb"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 197: a city nobody was watching laid the whole city out again
// for every snapshot the daemon sent, at 7.9% of a core on the real
// history against the budget in performance/budgets.md: hidden,
// minimised or unfocused, at most 1% of one core. A hidden city now
// holds its snapshots and stays subscribed, since the daemon sends only
// what changed (item 201); an unfocused one that can be seen is a
// dashboard (Aria, 2026-10-21), laying out each snapshot and painting
// one frame for it.
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
	performanceTier(t)
	const (
		stream = 15 * time.Second
		// storeStream is long enough to hold two of the ledger's 30 s
		// refreshes, which a 15 s stream can fall between (item 215).
		storeStream   = 65 * time.Second
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
		// store is whether the daemon keeps, and the city reads, a
		// statistics store holding a history the real one's size: the
		// city's cost breakdown reads it while it lays out (item 215).
		store bool
	}{
		{"hidden", helpers.Hide, "visible=false", false, false},
		{"unfocused, and visible", helpers.Unfocus, "focused=false visible=true minimised=false", true, false},
		{"unfocused, and visible, over a statistics store the real history's size", helpers.Unfocus, "focused=false visible=true minimised=false", true, true},
	} {
		over := stream
		if way.store {
			over = storeStream
		}
		t.Run(fmt.Sprintf("when the city is %s, and a session of 200 writes a line every 200 ms for %.0f s", way.name, over.Seconds()), func(t *testing.T) {
			b := newBusyCity
			if way.store {
				b = newBusyCityWithStore
			}
			busy := b(t)
			root := busy.root
			roads := busy.roadsSeen(t)

			display := helpers.Display(t, render.DefaultWidth, render.DefaultHeight)
			city := exec.Command(botropolis, busy.cityArgs()...)
			city.Env = append(append(cleanEnv(), busy.cityEnv()...), "TMPDIR="+root, "BOTROPOLIS_FRAMETIME=1")
			city.Env = append(city.Env, display.Env()...)
			var said syncBuffer
			city.Stderr = &said
			require.NoError(t, city.Start())
			t.Cleanup(func() { _ = city.Process.Kill(); _ = city.Wait() })
			// The city is running once it has drawn its first frame. Its
			// frame timer's first report comes after 120, which a virtual
			// display draws in software and a busy machine stretched past
			// a minute (item 209).
			require.Eventually(t, func() bool {
				s := said.String()
				return strings.Contains(s, "DRAWN ") || strings.Contains(s, "QUIET ")
			}, 60*time.Second, 100*time.Millisecond)
			time.Sleep(3 * time.Second)
			way.do(t, display)
			// Its first frames are slow in software, and a busy machine
			// slower, so the window's state counts once the city says it
			// is in it, with a snapshot laid out if it draws.
			require.Eventually(t, func() bool {
				lines := strings.Split(strings.TrimSpace(said.String()), "\n")
				c, ok := lastQuiet(said.String())
				return ok && strings.Contains(lines[len(lines)-1], way.said) && (!way.drawn || c.snapshots > 0)
			}, 60*time.Second, 100*time.Millisecond, "the city never said its window was %s", way.name)
			time.Sleep(2 * time.Second)

			heard := len(said.String())
			counted, _ := lastQuiet(said.String())
			raster, rest := threadTicks(t, city.Process.Pid)
			read := bytesRead(t, city.Process.Pid)
			process, uncountable := measure.Process(city.Process.Pid)
			_, system := cpuTicksSplit(t, city.Process.Pid)
			elapsed := busy.write(every, over)
			raster1, rest1 := threadTicks(t, city.Process.Pid)
			_, system1 := cpuTicksSplit(t, city.Process.Pid)
			var retired uint64
			if uncountable == nil {
				retired = process.Instructions()
				_ = process.Close()
			}
			read = bytesRead(t, city.Process.Pid) - read
			time.Sleep(1500 * time.Millisecond)
			counted1, _ := lastQuiet(said.String())
			fed := counted1.received - counted.received
			share := func(ticks int64) float64 { return float64(ticks) / clockTicksSec / elapsed.Seconds() }
			rasterShare, restShare := share(raster1-raster), share(rest1-rest)
			own := (counted1.decoding + counted1.layout - counted.decoding - counted.layout).Seconds() / elapsed.Seconds()
			during := said.String()[heard:]
			t.Logf("city %s: decoding and laying out %d snapshots %.2f%% of a core over %.0f s (decoding %.0f ms, laying out %.0f ms); process %.2f%% outside the software rasteriser, %.2f%% in it; %d KiB from the daemon, %d KiB read in all",
				way.name, counted1.snapshots-counted.snapshots, 100*own, elapsed.Seconds(),
				ms(counted1.decoding-counted.decoding), ms(counted1.layout-counted.layout), 100*restShare, 100*rasterShare, fed>>10, read>>10)
			// What the gate measures is instructions, which a busy machine
			// cannot move, with the time spent in the kernel, which the
			// counters cannot see, and CPU only where they cannot be
			// counted (item 209). Unfocused, the city counts its own;
			// hidden, the test counts the whole process, which draws
			// nothing.
			measured, inInstructions := restShare+rasterShare, uncountable == nil
			if inInstructions {
				measured = float64(retired)/measure.CoreRate/elapsed.Seconds() + share(system1-system)
			}
			if way.drawn {
				measured, inInstructions = own, counted1.counted
			}
			t.Logf("gated: %.2f%% of a core, counted in instructions: %v", 100*measured, inInstructions)

			// The city lays out the roads the real history has; a fixture
			// without them measured a fifth of what laying out costs (item 206).
			t.Run(fmt.Sprintf("the busy city should have its %d roads", busy.roads), func(t *testing.T) {
				assert.Equal(t, busy.roads, roads)
			})

			// A city that cannot read the store counts its breakdown from
			// memory, which costs nothing like reading it (item 215).
			if way.store {
				t.Run("the city should have read the statistics store", func(t *testing.T) {
					assert.True(t, holdsOpen(t, city.Process.Pid, filepath.Join(busy.state, "botropolis", "stats.db")))
				})
			}

			if way.drawn {
				t.Run("the daemon's updates should have reached it", func(t *testing.T) {
					assert.Greater(t, counted1.snapshots-counted.snapshots, 5)
				})
			} else {
				// Hidden, it holds what it is sent without laying it out, so
				// what it read from the daemon says it was streamed: some,
				// and no whole snapshot of about 220 KiB. What it reads in
				// all, from the X server and its files too, a busy machine
				// moves (item 209).
				t.Run("the daemon should have streamed it what changed", func(t *testing.T) {
					assert.Greater(t, fed, int64(1<<10))
				})

				t.Run("it should have read no whole snapshot", func(t *testing.T) {
					assert.Less(t, fed, int64(64<<10))
				})
			}

			t.Run("the window should have been "+way.name+" throughout", func(t *testing.T) {
				require.Contains(t, during, "QUIET no frames: ")
				for _, line := range strings.Split(during, "\n") {
					if strings.HasPrefix(line, "QUIET no frames: ") && !strings.Contains(line, way.said) {
						t.Fatalf("the window was not %s: %s", way.name, line)
					}
				}
			})

			t.Run("its instructions, but for drawing, should be at most 1% of a core", func(t *testing.T) {
				if !inInstructions {
					t.Skip("skipped: no hardware instruction counters here, so the thread CPU check below gates instead, and a busy machine moves it")
				}
				// Unfocused, counted with its time in the kernel, it read
				// 1.04% on a quiet machine (item 209), and 0.82-0.83% when
				// item 214 made this a plain gate.
				assert.LessOrEqual(t, measured, maxCoreShare)
			})

			if !inInstructions {
				t.Run("without instruction counters, its CPU, but for drawing, should be at most 1% of a core", func(t *testing.T) {
					assert.LessOrEqual(t, measured, maxCoreShare)
				})
			}
		})
	}
}

// quietCount is what the frame timer's last QUIET line said the city's
// snapshots have cost it.
type quietCount struct {
	snapshots        int
	decoding, layout time.Duration
	// received is the bytes read from the daemon.
	received int64
	// counted is whether its costs were counted in instructions.
	counted bool
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
		var how string
		if _, err := fmt.Sscanf(lines[i][at:], " snapshots=%d decoding=%fms layout=%fms received=%dB counted=%s", &c.snapshots, &decoding, &layout, &c.received, &how); err != nil {
			continue
		}
		c.decoding = time.Duration(decoding * float64(time.Millisecond))
		c.layout = time.Duration(layout * float64(time.Millisecond))
		c.counted = how == "instructions"
		return c, true
	}
	return quietCount{}, false
}

// holdsOpen is whether a process holds the file at path open.
func holdsOpen(t *testing.T, pid int, path string) bool {
	t.Helper()
	dir := filepath.Join("/proc", strconv.Itoa(pid), "fd")
	fds, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, fd := range fds {
		if to, err := os.Readlink(filepath.Join(dir, fd.Name())); err == nil && to == path {
			return true
		}
	}
	return false
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
