//go:build linux

package acceptance_test

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"botropolis.city/botropolis/pkg/measure"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// TestCalibrateCoreRate measures measure.CoreRate's two rates, run by
// tools/calibrate-core-rate on an unloaded machine (item 209): the
// daemon's load of 2,000 small sessions, on one thread pinned to the
// first core of each kind, instructions retired over thread CPU.
func TestCalibrateCoreRate(t *testing.T) {
	if os.Getenv("BOTROPOLIS_CALIBRATE") == "" {
		t.Skip("run by tools/calibrate-core-rate")
	}
	if err := measure.Available(); err != nil {
		t.Fatalf("no hardware instruction counters: %v", err)
	}
	measure.CountInstructions.Store(true)
	defer measure.CountInstructions.Store(false)

	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	home := helpers.NewHome(t)
	started := map[string]bool{}
	for _, p := range helpers.ManySessions(198, 2000, now) {
		if !started[p.Session] {
			started[p.Session] = true
			ts := p.At.Format("2006-01-02T15:04:05.000Z")
			home.Transcript(p.Session, p.Dir, helpers.UserPrompt(p.Session, p.Dir, ts),
				helpers.AssistantReply(p.Session, "msg_"+p.Session[len(p.Session)-6:], ts))
		}
		home.Prompt(p.Session, p.Dir, p.Text, p.At)
	}
	loader := state.NewLoader(home.Path, state.Probes{Alive: func(int) bool { return false }, Attached: func(string) bool { return false }})
	_, err := loader.Load(now)
	require.NoError(t, err)

	t.Logf("load average: %s", strings.TrimSpace(string(must(os.ReadFile("/proc/loadavg")))))

	// In bursts, as the daemon works: one load every 200 ms, sleeping in
	// between, on whichever core the scheduler gives it, which is idling
	// at a low clock when each burst starts.
	var bursts measure.Total
	at := now
	for range 50 {
		at = at.Add(time.Second)
		time.Sleep(200 * time.Millisecond)
		bursts.Add(measure.Thread(func() {
			_, err := loader.Load(at)
			require.NoError(t, err)
		}))
	}
	b := bursts.Cost()
	t.Logf("in bursts every 200 ms, unpinned: %d instructions in %v of thread CPU: %.4g instructions a second",
		b.Instructions, b.CPU, float64(b.Instructions)/b.CPU.Seconds())

	for _, kind := range []string{"cpu_core", "cpu_atom"} {
		cpus := strings.TrimSpace(string(must(os.ReadFile("/sys/devices/" + kind + "/cpus"))))
		first, _, _ := strings.Cut(strings.Split(cpus, ",")[0], "-")
		cpu, err := strconv.Atoi(first)
		require.NoError(t, err)

		runtime.LockOSThread()
		var set unix.CPUSet
		set.Set(cpu)
		require.NoError(t, unix.SchedSetaffinity(0, &set))
		var total measure.Total
		var clocks []string
		at := now
		for range 20 {
			at = at.Add(time.Second)
			runtime.GC()
			total.Add(measure.Thread(func() {
				_, err := loader.Load(at)
				require.NoError(t, err)
			}))
			clocks = append(clocks, strings.TrimSpace(string(must(os.ReadFile("/sys/devices/system/cpu/cpu"+strconv.Itoa(cpu)+"/cpufreq/scaling_cur_freq")))))
		}
		runtime.UnlockOSThread()
		c := total.Cost()
		t.Logf("%s, CPU %d: %d instructions in %v of thread CPU: %.4g instructions a second; its clock in kHz after each load: %s",
			kind, cpu, c.Instructions, c.CPU, float64(c.Instructions)/c.CPU.Seconds(), strings.Join(clocks, " "))
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
