//go:build linux

package acceptance_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/app"
	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/demo"
	"botropolis.city/botropolis/pkg/measure"
	"botropolis.city/botropolis/pkg/render"
	"botropolis.city/botropolis/testing/helpers"
)

// bigHistory is a made-up history at the scale of the laptop item 270
// was reported from: 1,600 sessions in 600 projects, and a burst of 800
// scripted runs in 300 temporary directories that all name one ticket,
// so they make a group of 800. Nothing in it comes from a real history.
func bigHistory(t *testing.T, now time.Time) string {
	t.Helper()
	history, err := demo.SynthHistory(270, 1600, 600, now)
	require.NoError(t, err)
	burst := now.Add(-3 * 24 * time.Hour)
	for i := range 800 {
		history = append(history, demo.HistoryPrompt{
			Session: fmt.Sprintf("00000000-0000-4270-8000-%012d", i),
			Dir:     fmt.Sprintf("/tmp/tmp.burst%04d", i%300),
			Text:    "please run the nightly check for TICKET-270",
			At:      burst.Add(time.Duration(i) * 50 * time.Second),
		})
	}
	sort.SliceStable(history, func(i, j int) bool { return history[i].At.Before(history[j].At) })
	home := t.TempDir()
	_, err = demo.WriteHome(home, history, "2006-01-02T15:04:05.000Z")
	require.NoError(t, err)
	return home
}

var frameLine = regexp.MustCompile(`(?m)^FRAME \d+ frames: ([0-9.]+) ms/frame`)

// framesMS is every FRAME line's drawing time a frame, in order.
func framesMS(log string) []float64 {
	var out []float64
	for _, m := range frameLine.FindAllStringSubmatch(log, -1) {
		ms, _ := strconv.ParseFloat(m[1], 64)
		out = append(out, ms)
	}
	return out
}

// Item 270: a long history on another laptop opened a city of 559
// districts and groups of up to 781, which drew a frame a second and
// would not quit. The city at that scale has to open and settle within
// the City budgets (performance/budgets.md in the meta repository).
//
// Aria set its bars on 2026-10-26: every city's budgets, and 10 ms of CPU
// a frame zoomed in or panning, with 16 ms, one frame at 60 Hz, for the
// fitted view, where every building is on screen.
func TestACityOfSixHundredDistrictsFrames(t *testing.T) {
	performanceTier(t)
	gate := stressGateFor(os.Getenv)
	t.Logf("the stress city %s", gate.mode)
	const (
		fitted = 16 * time.Millisecond
		closer = 10 * time.Millisecond
		frames = 60
	)
	home := bigHistory(t, time.Now())
	d, err := render.NewDrive(render.DriveOptions{Home: home, Width: 1600, Height: 900})
	require.NoError(t, err)
	// Item 315: the collector's own goal is twice the live heap, so a
	// live heap over half the city's limit leaves the limit, not the
	// goal, driving every collection; at the limit it ran flat out.
	const liveBar = app.CityMemoryLimit / 2
	var live []uint64
	for range 3 {
		require.NoError(t, d.Tick())
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		live = append(live, m.HeapAlloc)
	}
	t.Logf("live Go heap after each of the first frames: %v MiB, the bar %d MiB", mib(live), liveBar>>20)

	t.Run("when the city of 600 districts draws its first three frames", func(t *testing.T) {
		t.Run("it should hold its live Go heap at or under 150 MiB, half the city's 300 MiB limit, so the collector's own goal stays under the limit", func(t *testing.T) {
			holdToBar(t, gate, slices.Max(live) <= liveBar, "live Go heap %v MiB, the bar %d MiB", mib(live), liveBar>>20)
		})
	})

	require.NoError(t, d.Run(settleTicks))
	// median is the middle CPU a tick took of frames ticks, each after
	// step.
	median := func(step func()) time.Duration {
		cpu := make([]time.Duration, 0, frames)
		for range frames {
			step()
			cpu = append(cpu, measure.Thread(func() { require.NoError(t, d.Tick()) }).CPU)
		}
		slices.Sort(cpu)
		return cpu[frames/2]
	}

	t.Run("when the city of 600 districts is fitted", func(t *testing.T) {
		took := median(func() {})
		t.Logf("fitted: %v of CPU a frame, the bar %v", took, fitted)

		t.Run("it should take at most 16 ms of CPU a frame", func(t *testing.T) {
			holdToBar(t, gate, took <= fitted, "fitted took %v, the bar %v", took, fitted)
		})
	})

	t.Run("when it is zoomed in to full size", func(t *testing.T) {
		for d.Shown().Zoom < 1 {
			require.NoError(t, d.WheelAt(city.Point{X: 800, Y: 450}, 1))
			require.NoError(t, d.Run(3))
		}
		require.NoError(t, d.Run(settleTicks))
		took := median(func() {})
		t.Logf("zoomed in: %v of CPU a frame, the bar %v", took, closer)

		t.Run("it should take at most 10 ms of CPU a frame", func(t *testing.T) {
			holdToBar(t, gate, took <= closer, "zoomed in took %v, the bar %v", took, closer)
		})

		t.Run("and it is panned", func(t *testing.T) {
			require.NoError(t, d.Press("hold+ArrowRight"))
			took := median(func() {})
			t.Logf("panning: %v of CPU a frame, the bar %v", took, closer)

			t.Run("it should take at most 10 ms of CPU a frame", func(t *testing.T) {
				holdToBar(t, gate, took <= closer, "panning took %v, the bar %v", took, closer)
			})
		})
	})
}

// mib is each of bytes in MiB.
func mib(bytes []uint64) []uint64 {
	out := make([]uint64, len(bytes))
	for i, b := range bytes {
		out[i] = b >> 20
	}
	return out
}

// firstMapLimit is how long the city of 600 districts has to draw its
// first frame where the test runs (Aria, 2026-10-27, item 303): 5 s on her
// machine, which item 270 set the bar for, and 9 s on CI, where a shared
// runner with no GPU measured 7.0 s for 0.3.3's release.
func firstMapLimit(getenv func(string) string) (time.Duration, string) {
	if getenv("CI") != "" {
		return 9 * time.Second, "9s, the CI limit: CI is set, and a shared runner measured 7.0 s (item 303)"
	}
	return 5 * time.Second, "5s, the reference machine's limit: CI is not set (item 270)"
}

// stressGate is whether the city of 600 districts holds its measures to
// their bars where it runs (Aria, 2026-10-27, item 305): a shared runner
// varies too much between runs to gate on, so with CI set every measure is
// logged with its bar, and the gate is the reference machine's quiet
// make test-performance, which every release runs.
type stressGate struct {
	gates bool
	mode  string
}

func stressGateFor(getenv func(string) string) stressGate {
	if getenv("CI") != "" {
		return stressGate{false, "reports and does not gate: CI is set, and a shared runner's CPU varies too much between runs to hold a bar (item 305); the reference machine's quiet make test-performance gates"}
	}
	return stressGate{true, "gates: CI is not set, so this is the reference machine"}
}

type barT interface {
	Helper()
	Errorf(string, ...any)
	Logf(string, ...any)
}

// holdToBar fails t when the measure is over its bar and g gates, and only
// logs it when g reports.
func holdToBar(t barT, g stressGate, ok bool, format string, args ...any) {
	t.Helper()
	hold(t, g, ok, "over its bar", format, args...)
}

// holdToFrames fails t when the city drew no frame to measure and g gates
// (item 306): a slow shared runner can draw none within the window, which
// the reference machine never does.
func holdToFrames(t barT, g stressGate, frames []float64) {
	t.Helper()
	hold(t, g, len(frames) > 0, "no frames measured", "the runner drew none within the window")
}

func hold(t barT, g stressGate, ok bool, what, format string, args ...any) {
	t.Helper()
	switch {
	case ok:
	case g.gates:
		t.Errorf(what+": "+format, args...)
	default:
		t.Logf(what+", reported and not gated: "+format, args...)
	}
}

func TestACityOfSixHundredDistricts(t *testing.T) {
	performanceTier(t)
	helpers.NeedXvfb(t)
	gate := stressGateFor(os.Getenv)
	t.Logf("the stress city %s", gate.mode)
	firstFrame, why := firstMapLimit(os.Getenv)
	t.Logf("first-map limit: %s", why)
	const (
		settled = 512 << 20
		peak    = 1 << 30
		settle  = 30 * time.Second
		window  = 5 * time.Second
	)

	t.Run("when a city of 2,400 sessions in 600 districts, its temporary directories in one, with a group of 800, opens headless on its daemon", func(t *testing.T) {
		r := newStateRun(t)
		home := bigHistory(t, time.Now())
		daemon, err := botropolisd()
		require.NoError(t, err)
		sock := shortSocket(t)
		d := exec.Command(daemon, "--home", home, "--socket", sock)
		d.Env = append(os.Environ(), "XDG_STATE_HOME="+filepath.Join(r.tmp, "daemon-state"), "PATH=/nonexistent")
		require.NoError(t, d.Start())
		t.Cleanup(func() { _ = d.Process.Kill(); _ = d.Wait() })
		require.Eventually(t, func() bool { _, err := os.Stat(sock); return err == nil }, time.Minute, 20*time.Millisecond)
		cmd := exec.Command(botropolis, "city", "--headless", "--home", home, "--socket", sock)
		cmd.Env = append(r.env(), "BOTROPOLIS_FRAMETIME=1", "GODEBUG=gctrace=1")
		var out syncBuffer
		cmd.Stdout, cmd.Stderr = &out, &out
		start := time.Now()
		require.NoError(t, cmd.Start())
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()

		var drawnIn time.Duration
		var top, last int64
		tick := time.NewTicker(100 * time.Millisecond)
		for time.Since(start) < settle {
			<-tick.C
			if drawnIn == 0 && strings.Contains(out.String(), "DRAWN the first frame") {
				drawnIn = time.Since(start)
			}
			var now int64
			for _, pid := range children(cmd.Process.Pid) {
				now = max(now, privateDirty(pid))
			}
			top = max(top, now)
			if time.Since(start) > settle-window {
				last = max(last, now)
			}
		}
		tick.Stop()
		stopCity(t, cmd, done)
		log := out.String()
		t.Logf("first frame after %v; Private_Dirty peaked at %d MiB and settled at %d MiB; Go heap peaked at %d MB, %d MB live at the end; ms a frame of drawing: %v",
			drawnIn, top>>20, last>>20, peakHeapMB(log), lastLiveMB(log), framesMS(log))

		t.Run("it should draw its first frame within "+firstFrame.String(), func(t *testing.T) {
			holdToBar(t, gate, drawnIn > 0 && drawnIn <= firstFrame, "first frame after %v, the limit %v", drawnIn, firstFrame)
		})

		t.Run("it should report its frames, or nothing was measured", func(t *testing.T) {
			holdToFrames(t, gate, framesMS(log))
		})

		t.Run("it should hold under 512 MiB of Private_Dirty once settled", func(t *testing.T) {
			holdToBar(t, gate, last < int64(settled), "settled at %d MiB, the bar 512 MiB", last>>20)
		})

		t.Run("it should never pass 1 GiB of Private_Dirty", func(t *testing.T) {
			holdToBar(t, gate, top < int64(peak), "peaked at %d MiB, the bar 1 GiB", top>>20)
		})
	})
}

// recordStressCity records the city of 600 districts headless on its own
// daemon, calls sample with the drawing child's pid every tenth of a
// second until the first frame is written or giveUp passes, the last
// time once the frame is there, and returns
// when the first frame came, zero for none, and what the city said.
func recordStressCity(t *testing.T, giveUp time.Duration, sample func(pid int)) (time.Duration, string) {
	t.Helper()
	r := newStateRun(t)
	home := bigHistory(t, time.Now())
	daemon, err := botropolisd()
	require.NoError(t, err)
	sock := shortSocket(t)
	d := exec.Command(daemon, "--home", home, "--socket", sock)
	d.Env = append(os.Environ(), "XDG_STATE_HOME="+filepath.Join(r.tmp, "daemon-state"), "PATH=/nonexistent")
	require.NoError(t, d.Start())
	t.Cleanup(func() { _ = d.Process.Kill(); _ = d.Wait() })
	require.Eventually(t, func() bool { _, err := os.Stat(sock); return err == nil }, time.Minute, 20*time.Millisecond)
	frames := filepath.Join(r.tmp, "frames")
	cmd := exec.Command(botropolis, "city", "--headless", "--home", home, "--socket", sock,
		"--record", frames, "--fps", "1", "--seconds", "60")
	cmd.Env = r.env()
	var out syncBuffer
	cmd.Stdout, cmd.Stderr = &out, &out
	start := time.Now()
	require.NoError(t, cmd.Start())
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	var firstIn time.Duration
	tick := time.NewTicker(100 * time.Millisecond)
	for firstIn == 0 && time.Since(start) < giveUp {
		<-tick.C
		if entries, _ := os.ReadDir(frames); len(entries) > 0 {
			firstIn = time.Since(start)
		}
		if kids := children(cmd.Process.Pid); len(kids) > 0 {
			sample(kids[len(kids)-1])
		}
	}
	tick.Stop()
	stopCity(t, cmd, done)
	return firstIn, out.String()
}

// Item 311: a recording of the city of 600 districts wrote no frame in
// twelve minutes: its first tick came before the daemon's first snapshot,
// and its clock then waited for a paint that the unopened city never made.
// The bound is a hang detector, not a target.
func TestARecordingCityOfSixHundredDistricts(t *testing.T) {
	helpers.NeedXvfb(t)
	const giveUp = 2 * time.Minute

	t.Run("when a recording city of 2,400 sessions in 600 districts opens headless on its daemon", func(t *testing.T) {
		firstIn, log := recordStressCity(t, giveUp, func(int) {})
		t.Logf("first recorded frame after %v; the city said:\n%s", firstIn, log)

		t.Run("it should write a frame", func(t *testing.T) {
			assert.NotZero(t, firstIn, "no frame within %v", giveUp)
		})
	})
}

// Item 311: the unopened recording spent about six cores drawing its
// opening. Opening costs building the scene and one fitted frame, baking
// every look cold, and the wait for the daemon nothing but the log's
// progress line, so the city's own process has 2.5 CPU-seconds to its
// first frame headless (performance/budgets.md, First launch; Aria,
// 2026-10-28, item 315: "2.5 CPU-s"), and firstMapLimit to get there.
func TestARecordingCityOfSixHundredDistrictsOpening(t *testing.T) {
	performanceTier(t)
	helpers.NeedXvfb(t)
	gate := stressGateFor(os.Getenv)
	t.Logf("the stress city %s", gate.mode)
	firstFrame, why := firstMapLimit(os.Getenv)
	t.Logf("first-map limit: %s", why)
	const (
		giveUp        = 2 * time.Minute
		budget        = 2500 * time.Millisecond
		clockTicksSec = 100
	)

	t.Run("when a recording city of 2,400 sessions in 600 districts opens headless on its daemon", func(t *testing.T) {
		var ticks int64
		firstIn, log := recordStressCity(t, giveUp, func(pid int) { ticks = cpuTicks(t, pid) })
		spent := time.Duration(ticks) * time.Second / clockTicksSec
		t.Logf("first recorded frame after %v, %v of the city's CPU; the city said:\n%s", firstIn, spent, log)

		t.Run("it should write its first frame within "+firstFrame.String(), func(t *testing.T) {
			holdToBar(t, gate, firstIn > 0 && firstIn <= firstFrame, "first recorded frame after %v (0 is none within %v), the limit %v", firstIn, giveUp, firstFrame)
		})

		t.Run("it should spend at most 2.5 CPU-seconds before its first frame", func(t *testing.T) {
			holdToBar(t, gate, firstIn > 0 && spent <= budget, "%v of CPU to the first frame (written after %v), the bar %v", spent, firstIn, budget)
		})
	})
}
