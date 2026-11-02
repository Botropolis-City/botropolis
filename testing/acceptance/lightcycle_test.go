//go:build linux

package acceptance_test

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/script"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 308: the switch to night costs about 49 MiB, within the settled
// budgets only if it is paid once. The city of 600 districts follows its
// clock, scrubbed day → night → day → night → day, and each light's
// settled memory is held to the same light's before it. A phase is 180 s
// and judged on its last 30 s: 40 s phases had not settled, and read a
// ratchet as memory given back.
//
// Item 318: under llvmpipe the process's Private_Dirty ratchets with
// memory the C allocator keeps after the driver has freed it, which a
// city on a real GPU does not pay, so headless the check counts what the
// city holds itself: its images and its live Go heap. The whole process
// is judged on a real GPU.
const (
	cyclePhase   = 180
	cycleSettled = 30
	cycleMargin  = 4 << 20
)

// cyclePresses scrub the clock, which starts at noon, to the next light
// at each phase's start, 22:00 for night and 10:00 for day, an hour a
// press.
var (
	cyclePresses = []int{0, 10, 12, 12, 12}
	cycleNames   = []string{"warm-up day", "first night", "first day", "second night", "second day"}
)

// cycleReading is one settled reading, per measure, of the city.
type cycleReading map[string]int64

// lightCycle runs the cycle and returns each phase's mean over its
// settled seconds, per measure. read is asked once a second within the
// settled window with the city's processes and returns what it measures
// then, without a measure it has nothing new for.
func lightCycle(t *testing.T, windowed bool, env []string, read func(pids []int) cycleReading) []cycleReading {
	t.Helper()
	r := newStateRun(t)
	home := bigHistory(t, time.Now())
	daemon, err := botropolisd()
	require.NoError(t, err)
	sock := shortSocket(t)
	d := exec.Command(daemon, "--home", home, "--socket", sock)
	d.Env = append(os.Environ(), "XDG_STATE_HOME="+filepath.Join(r.tmp, "daemon-state"), "PATH=/nonexistent")
	d.SysProcAttr = tiedToTest()
	require.NoError(t, d.Start())
	t.Cleanup(func() { _ = d.Process.Kill(); _ = d.Wait() })
	require.Eventually(t, func() bool { _, err := os.Stat(sock); return err == nil }, time.Minute, 20*time.Millisecond)

	var cues []script.Cue
	for i, n := range cyclePresses {
		for k := 0; k < n; k++ {
			cues = append(cues, script.Cue{At: float64(i*cyclePhase) + 0.2*float64(k), Key: "BracketRight"})
		}
	}
	cueFile := filepath.Join(r.tmp, "cycle.json")
	require.NoError(t, script.Save(cueFile, cues))
	frames := filepath.Join(r.tmp, "frames")
	args := []string{"city", "--home", home, "--socket", sock, "--set", "light=live",
		"--record", frames, "--fps", "1", "--seconds", strconv.Itoa(len(cyclePresses) * cyclePhase), "--script", cueFile}
	cmdEnv := append(append(r.env(), "TZ="+helpers.ZoneAt(12, time.Now())), env...)
	if windowed {
		cmdEnv = append(cmdEnv, "DISPLAY="+os.Getenv("DISPLAY"), "XAUTHORITY="+os.Getenv("XAUTHORITY"))
	} else {
		args = append([]string{args[0], "--headless"}, args[1:]...)
	}
	cmd := exec.Command(botropolis, args...)
	cmd.Env = cmdEnv
	cmd.SysProcAttr = tiedToTest()
	var out syncBuffer
	cmd.Stdout, cmd.Stderr = &out, &out
	require.NoError(t, cmd.Start())
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	sums := make([]cycleReading, len(cyclePresses))
	counts := make([]cycleReading, len(cyclePresses))
	for i := range sums {
		sums[i], counts[i] = cycleReading{}, cycleReading{}
	}
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	started := time.Now()
sample:
	for {
		select {
		case err := <-done:
			require.NoError(t, err, out.String())
			break sample
		case <-tick.C:
		}
		at := countFiles(frames)
		if at == 0 && time.Since(started) > firstFrameWait {
			stopCity(t, cmd, done)
			require.FailNow(t, "the city wrote no frame", "after %v; it said:\n%s", firstFrameWait, out.String())
		}
		i := at / cyclePhase
		if i >= len(cyclePresses) || at%cyclePhase < cyclePhase-cycleSettled {
			continue
		}
		pids := []int{cmd.Process.Pid}
		if !windowed {
			pids = children(cmd.Process.Pid)
		}
		for k, v := range read(pids) {
			sums[i][k] += v
			counts[i][k]++
		}
	}
	means := make([]cycleReading, len(cyclePresses))
	for i := range cyclePresses {
		require.NotEmpty(t, counts[i], "no samples settled in the %s", cycleNames[i])
		means[i] = cycleReading{}
		var parts []string
		for k, n := range counts[i] {
			means[i][k] = sums[i][k] / n
			parts = append(parts, k+" "+strconv.FormatInt(means[i][k]>>20, 10)+" MiB")
		}
		t.Logf("%s settled at %s", cycleNames[i], strings.Join(parts, ", "))
	}
	return means
}

func TestLightCycleGivesItsMemoryBack(t *testing.T) {
	performanceTier(t)
	helpers.NeedXvfb(t)
	gate := stressGateFor(os.Getenv)
	t.Logf("the light cycle %s", gate.mode)

	t.Run("when the city of 600 districts goes night, day, night and day by its scrubbed clock", func(t *testing.T) {
		memory := filepath.Join(t.TempDir(), "memory.log")
		var lines int
		means := lightCycle(t, false, []string{"BOTROPOLIS_MEMORY_LOG=" + memory}, func(pids []int) cycleReading {
			reading := cycleReading{"Private_Dirty": 0}
			for _, pid := range pids {
				reading["Private_Dirty"] = max(reading["Private_Dirty"], privateDirty(pid))
			}
			fresh := memoryLines(memory)
			if len(fresh) > lines {
				last := fresh[len(fresh)-1]
				reading["images"] = last["images"]
				if heap, ok := last["heap"]; ok {
					if slack, ok := last["slack"]; ok {
						reading[heapLessSlack] = heap - slack
					}
				}
				lines = len(fresh)
			}
			return reading
		})
		measures := []string{"images", heapLessSlack}
		for i, phase := range means {
			for _, measure := range measures {
				require.Contains(t, phase, measure, "the city logged no %s in the %s", measure, cycleNames[i])
			}
		}
		require.Empty(t, memoryBroken(memory), "a half of the record held more than its rule allows, so its slack cannot be subtracted")

		for _, light := range []struct {
			name          string
			first, second int
		}{
			{"night", 1, 3},
			{"day", 2, 4},
		} {
			for _, measure := range measures {
				t.Run("it should settle the second "+light.name+"'s "+measure+" within 4 MiB of the first", func(t *testing.T) {
					holdCycle(t, gate, light.name, measure, means[light.first][measure], means[light.second][measure])
				})
			}
		}
	})
}

func TestLightCycleOnARealGPU(t *testing.T) {
	performanceTier(t)
	if os.Getenv("BOTROPOLIS_REAL_GPU") != "1" {
		t.Skip("set BOTROPOLIS_REAL_GPU=1 to cycle the light windowed on this machine's GPU (item 318)")
	}
	require.NotEmpty(t, os.Getenv("DISPLAY"), "the real-GPU cycle opens a window, and there is no DISPLAY")
	gate := stressGateFor(os.Getenv)
	t.Logf("the light cycle %s", gate.mode)

	t.Run("when the city goes night, day, night and day on a real GPU", func(t *testing.T) {
		means := lightCycle(t, true, nil, func(pids []int) cycleReading {
			gpu := drmTotal(pids[0])
			return cycleReading{"Private_Dirty and GPU": privateDirty(pids[0]) + gpu, "GPU": gpu}
		})
		for i, phase := range means {
			require.Positive(t, phase["GPU"], "the city held no GPU memory in the %s, so this is not a real GPU", cycleNames[i])
		}

		for _, light := range []struct {
			name          string
			first, second int
		}{
			{"night", 1, 3},
			{"day", 2, 4},
		} {
			t.Run("it should settle the second "+light.name+" within 4 MiB of the first", func(t *testing.T) {
				holdCycle(t, gate, light.name, "Private_Dirty and GPU", means[light.first]["Private_Dirty and GPU"], means[light.second]["Private_Dirty and GPU"])
			})
		}
	})
}

// holdCycle holds the second light's settled measure to within the
// margin of the first's where g gates, and reports it where g does not
// (item 329).
func holdCycle(t barT, g stressGate, light, measure string, first, second int64) {
	t.Helper()
	holdToBar(t, g, second-first <= cycleMargin, "the second %s's %s settled at %d MiB against the first's %d MiB, the margin %d MiB",
		light, measure, second>>20, first>>20, cycleMargin>>20)
}

// tiedToTest kills a process the cycle starts when the test's own
// process dies: a test that times out panics past its cleanups, and on
// 2026-10-28 the cycle's daemon outlived one for four days, holding the
// landing lock its test had inherited (item 318).
func tiedToTest() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
}

// heapLessSlack is the live Go heap less the slack the record's two
// halves hold, which recordsize.go permits and which moves from run to
// run with no growth behind it (item 318).
const heapLessSlack = "heap less the records' slack"

// memoryBroken is the first half and buffer of the record the city's
// memory log names as having broken its rule, or nothing.
func memoryBroken(path string) string {
	data, _ := os.ReadFile(path)
	for _, field := range strings.Fields(string(data)) {
		if half, ok := strings.CutPrefix(field, "broken="); ok {
			return half
		}
	}
	return ""
}

// memoryLines reads the city's memory log, one reading a line.
func memoryLines(path string) []cycleReading {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	var out []cycleReading
	s := bufio.NewScanner(f)
	for s.Scan() {
		reading := cycleReading{}
		for _, field := range strings.Fields(s.Text()) {
			k, v, ok := strings.Cut(field, "=")
			if n, err := strconv.ParseInt(v, 10, 64); ok && err == nil {
				reading[k] = n
			}
		}
		out = append(out, reading)
	}
	return out
}

// drmTotal is the GPU memory a process's DRM clients hold, in bytes, as
// performance/tools/perf.py reads gpu_kib: drm-total-* summed over every
// region of each distinct client.
func drmTotal(pid int) int64 {
	dir := filepath.Join("/proc", strconv.Itoa(pid), "fd")
	fds, _ := os.ReadDir(dir)
	clients := map[string]int64{}
	for _, fd := range fds {
		if link, err := os.Readlink(filepath.Join(dir, fd.Name())); err != nil || !strings.HasPrefix(link, "/dev/dri/") {
			continue
		}
		data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "fdinfo", fd.Name()))
		if err != nil {
			continue
		}
		var driver, client string
		var total int64
		for _, line := range strings.Split(string(data), "\n") {
			key, value, _ := strings.Cut(line, ":")
			key, value = strings.TrimSpace(key), strings.TrimSpace(value)
			switch {
			case key == "drm-driver":
				driver = value
			case key == "drm-client-id":
				client = value
			case strings.HasPrefix(key, "drm-total-"):
				amount, unit, _ := strings.Cut(value, " ")
				n, _ := strconv.ParseInt(amount, 10, 64)
				total += n * drmUnits[unit]
			}
		}
		if client != "" {
			clients[driver+"/"+client] = total
		}
	}
	var sum int64
	for _, v := range clients {
		sum += v
	}
	return sum
}

var drmUnits = map[string]int64{"": 1, "B": 1, "KiB": 1 << 10, "kB": 1 << 10, "MiB": 1 << 20, "GiB": 1 << 30}

// firstFrameWait is how long the city of 600 districts has to write its
// first recorded frame.
const firstFrameWait = 2 * time.Minute

func countFiles(dir string) int {
	entries, _ := os.ReadDir(dir)
	return len(entries)
}
