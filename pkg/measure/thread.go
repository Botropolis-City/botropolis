package measure

import (
	"runtime"
	"sync/atomic"
	"time"

	"golang.org/x/sys/unix"
)

// What a "core" is, for the cost gates (item 209; Aria, 2026-10-22).
//
// CPU time is not a fixed measure of work on a laptop's hybrid chip: the
// same work costs several times more of it on an efficiency core than on
// a performance core, and more again when a busy package lowers every
// clock. So the gates count the instructions work retires, which neither
// moves, and turn them into time at the rate one core retires them.
//
// A core is a core as bursty work gets it (Aria, 2026-10-23). The work
// the gates measure wakes for a few milliseconds and sleeps again, on
// cores idling near 400 MHz that have not ramped up by the time it is
// done, so it retires instructions at half the rate sustained work does.
// Converted at a sustained rate, every gate read 2-7 times cheaper than
// the CPU its work takes.
//
// BurstCoreRate was measured with tools/calibrate-core-rate --wait on an
// Intel Core Ultra 7 265U on 2026-10-23 at 16:04 UTC, at a load average
// of 0.9 with no landing running: the daemon's load of 2,000 small
// sessions run as the daemon runs it, 50 times, once every 200 ms with
// sleeps between, unpinned, instructions retired over thread CPU.
//
// The sustained rates are kept for the record, measured the same way at
// 13:49 UTC at a load of 1.2: the same load 20 times back to back on one
// thread pinned to the first core of each kind, each retiring 900 M
// instructions. The performance core (CPU 0) ran at 4.6-4.8 GHz; the
// efficiency core (CPU 4) at 3.8-3.9 GHz once it had ramped up from
// 1.4 GHz over its first two loads. CoreRate was the efficiency core's
// rate until bursts were measured.
const (
	PerformanceCoreRate = 6.039e9
	EfficiencyCoreRate  = 4.370e9
	BurstCoreRate       = 2.025e9
	CoreRate            = BurstCoreRate
)

// CountInstructions turns on counting the instructions measured work
// retires. It opens hardware counters for every measure, so only a run
// that reports its costs turns it on.
var CountInstructions atomic.Bool

// Cost is what a piece of work cost the thread that ran it.
type Cost struct {
	CPU time.Duration
	// Kernel is the part of CPU spent in the kernel.
	Kernel       time.Duration
	Instructions uint64
	// Counted is whether its instructions were counted.
	Counted bool
}

// Core is the cost as time on a core: its user-space instructions at
// CoreRate, and its time in the kernel as it was spent, when they were
// counted, and its CPU time when not. The counters see only user space
// (perf_event_paranoid 2), and the budgets are all of a process's CPU,
// so the kernel's time is added; it still moves with a busy machine's
// clocks, but it is a small part (item 209).
func (c Cost) Core() time.Duration {
	if !c.Counted {
		return c.CPU
	}
	return time.Duration(float64(c.Instructions)/CoreRate*float64(time.Second)) + c.Kernel
}

// Total sums costs from any goroutine.
type Total struct {
	cpu, kernel, instructions, uncounted atomic.Int64
}

func (t *Total) Add(c Cost) {
	t.cpu.Add(int64(c.CPU))
	t.kernel.Add(int64(c.Kernel))
	t.instructions.Add(int64(c.Instructions))
	if !c.Counted {
		t.uncounted.Add(1)
	}
}

// Cost is the sum, counted only if every cost in it was.
func (t *Total) Cost() Cost {
	return Cost{
		CPU:          time.Duration(t.cpu.Load()),
		Kernel:       time.Duration(t.kernel.Load()),
		Instructions: uint64(t.instructions.Load()),
		Counted:      t.uncounted.Load() == 0,
	}
}

// Thread runs f locked to its OS thread and returns what that thread
// spent on it: its CPU, so time spent waiting for a core is not counted,
// and its instructions when CountInstructions is on and the machine can
// count them. Locking nests, so a goroutine already locked stays locked.
func Thread(f func()) Cost {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var counters *threadCounters
	if CountInstructions.Load() {
		counters, _ = openThread()
	}
	before, kernel := threadCPU(), threadKernel()
	start := counters.read()
	f()
	end := counters.read()
	c := Cost{CPU: threadCPU() - before, Kernel: threadKernel() - kernel}
	if counters != nil {
		c.Instructions, c.Counted = end-start, true
		counters.close()
	}
	return c
}

func threadCPU() time.Duration {
	var ts unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_THREAD_CPUTIME_ID, &ts); err != nil {
		return 0
	}
	return time.Duration(ts.Nano())
}
