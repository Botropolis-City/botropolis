package measure

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// pmus are the processor's core types: a hybrid chip has one counter
// unit per kind of core, and a thread counts only on the kind it is on.
var pmus = []string{"cpu_core", "cpu_atom", "cpu"}

// pmuTypes is the counter units this machine has, read once: the
// daemon opens counters on every rescan (item 213).
var pmuTypes = sync.OnceValues(readPMUTypes)

func readPMUTypes() ([]uint32, error) {
	var types []uint32
	for _, name := range pmus {
		b, err := os.ReadFile(filepath.Join("/sys/bus/event_source/devices", name, "type"))
		if err != nil {
			continue
		}
		n, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 32)
		if err != nil {
			return nil, err
		}
		if name == "cpu" {
			// One kind of core takes the plain event, with no unit named.
			n = 0
		}
		types = append(types, uint32(n))
	}
	if len(types) == 0 {
		return nil, errors.New("no processor counter units in /sys/bus/event_source/devices")
	}
	return types, nil
}

// open counts the user-space instructions one thread (and, with inherit,
// the threads it starts) retires on one kind of core.
func open(pmu uint32, tid int, inherit bool) (int, error) {
	attr := unix.PerfEventAttr{
		Type:   unix.PERF_TYPE_HARDWARE,
		Config: uint64(pmu)<<32 | unix.PERF_COUNT_HW_INSTRUCTIONS,
		Bits:   unix.PerfBitExcludeKernel | unix.PerfBitExcludeHv,
	}
	if inherit {
		attr.Bits |= unix.PerfBitInherit
	}
	attr.Size = uint32(unsafe.Sizeof(attr))
	return unix.PerfEventOpen(&attr, tid, -1, -1, unix.PERF_FLAG_FD_CLOEXEC)
}

type threadCounters struct {
	fds []int
}

func openAll(tid int, inherit bool) ([]int, error) {
	types, err := pmuTypes()
	if err != nil {
		return nil, err
	}
	fds := make([]int, 0, len(types))
	for _, pmu := range types {
		fd, err := open(pmu, tid, inherit)
		if err != nil {
			for _, fd := range fds {
				_ = unix.Close(fd)
			}
			return nil, fmt.Errorf("perf_event_open: %w", err)
		}
		fds = append(fds, fd)
	}
	return fds, nil
}

// openThread counts the calling thread, which must be locked to its
// goroutine.
func openThread() (*threadCounters, error) {
	fds, err := openAll(0, false)
	if err != nil {
		return nil, err
	}
	return &threadCounters{fds: fds}, nil
}

func sum(fds []int) uint64 {
	var total uint64
	var b [8]byte
	for _, fd := range fds {
		if _, err := unix.Read(fd, b[:]); err == nil {
			total += binary.LittleEndian.Uint64(b[:])
		}
	}
	return total
}

func (c *threadCounters) read() uint64 {
	if c == nil {
		return 0
	}
	return sum(c.fds)
}

func (c *threadCounters) close() {
	for _, fd := range c.fds {
		_ = unix.Close(fd)
	}
}

// threadKernel is the calling thread's time in the kernel.
func threadKernel() time.Duration {
	var u unix.Rusage
	if err := unix.Getrusage(unix.RUSAGE_THREAD, &u); err != nil {
		return 0
	}
	return time.Duration(u.Stime.Nano())
}

// Available says whether this machine can count instructions, and if
// not, why.
func Available() error {
	c, err := openThread()
	if err != nil {
		return err
	}
	c.close()
	return nil
}

// ProcessCounter counts the instructions a process retires, from when it is
// opened.
type ProcessCounter struct {
	fds []int
}

// Process counts the user-space instructions every thread of a process
// retires, and every thread they start, from now. The process must be
// one this process may trace, such as its child.
func Process(pid int) (*ProcessCounter, error) {
	tasks, err := os.ReadDir(filepath.Join("/proc", strconv.Itoa(pid), "task"))
	if err != nil {
		return nil, err
	}
	c := &ProcessCounter{}
	for _, task := range tasks {
		tid, err := strconv.Atoi(task.Name())
		if err != nil {
			continue
		}
		fds, err := openAll(tid, true)
		if err != nil {
			_ = c.Close()
			return nil, err
		}
		c.fds = append(c.fds, fds...)
	}
	return c, nil
}

// Instructions is what the process has retired since it was opened.
func (c *ProcessCounter) Instructions() uint64 {
	return sum(c.fds)
}

func (c *ProcessCounter) Close() error {
	for _, fd := range c.fds {
		_ = unix.Close(fd)
	}
	c.fds = nil
	return nil
}
